package orders

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type resolvedItem struct {
	MenuItemID     string
	ExternalItemID string
	Name           string
	UnitPrice      int64
	Quantity       int
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindByIdempotencyKey(ctx context.Context, customerID, key string) (Order, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id
		FROM orders
		WHERE customer_id = $1 AND idempotency_key = $2`, customerID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("query order by idempotency key: %w", err)
	}
	return loadOrder(ctx, r.pool, id)
}

func (r *PostgresRepository) CreatePending(ctx context.Context, command CreateCommand) (Order, bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Order{}, false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var restaurantStatus, integrationURL string
	err = tx.QueryRow(ctx, `
		SELECT status, integration_url
		FROM restaurants
		WHERE id = $1
		FOR SHARE`, command.RestaurantID).Scan(&restaurantStatus, &integrationURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, ErrRestaurantNotFound
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("lock restaurant: %w", err)
	}
	if restaurantStatus != "open" {
		return Order{}, false, ErrRestaurantUnavailable
	}

	items := make([]resolvedItem, 0, len(command.Items))
	var totalPrice int64
	for _, requested := range command.Items {
		var item resolvedItem
		var available bool
		err = tx.QueryRow(ctx, `
			SELECT id, external_item_id, name, price, is_available
			FROM menu_items
			WHERE id = $1 AND restaurant_id = $2
			FOR SHARE`, requested.MenuItemID, command.RestaurantID).
			Scan(&item.MenuItemID, &item.ExternalItemID, &item.Name, &item.UnitPrice, &available)
		if errors.Is(err, pgx.ErrNoRows) {
			return Order{}, false, ErrMenuItemNotFound
		}
		if err != nil {
			return Order{}, false, fmt.Errorf("lock menu item: %w", err)
		}
		if !available {
			return Order{}, false, ErrMenuItemUnavailable
		}
		if item.UnitPrice > (math.MaxInt64-totalPrice)/int64(requested.Quantity) {
			return Order{}, false, ErrInvalidRequest
		}
		item.Quantity = requested.Quantity
		totalPrice += item.UnitPrice * int64(item.Quantity)
		items = append(items, item)
	}

	orderID := uuid.NewString()
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (id, customer_id, restaurant_id, status, total_price, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (customer_id, idempotency_key) DO NOTHING
		RETURNING id`, orderID, command.CustomerID, command.RestaurantID, StatusPending, totalPrice, command.IdempotencyKey).
		Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return Order{}, false, fmt.Errorf("rollback idempotency conflict: %w", rollbackErr)
		}
		existing, findErr := r.FindByIdempotencyKey(ctx, command.CustomerID, command.IdempotencyKey)
		return existing, false, findErr
	}
	if err != nil {
		return Order{}, false, fmt.Errorf("insert order: %w", err)
	}

	for _, item := range items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (
				order_id, restaurant_id, menu_item_id, external_item_id, name, unit_price, quantity
			) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			orderID, command.RestaurantID, item.MenuItemID, item.ExternalItemID, item.Name, item.UnitPrice, item.Quantity)
		if err != nil {
			return Order{}, false, fmt.Errorf("insert order item: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, status, source)
		VALUES ($1, $2, 'system')`, orderID, StatusPending)
	if err != nil {
		return Order{}, false, fmt.Errorf("insert initial order status: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, false, fmt.Errorf("commit order: %w", err)
	}

	order, err := loadOrder(ctx, r.pool, orderID)
	if err != nil {
		return Order{}, false, err
	}
	order.RestaurantIntegrationURL = integrationURL
	return order, true, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (Order, error) {
	return loadOrder(ctx, r.pool, id)
}

func (r *PostgresRepository) ApplyRestaurantDecision(ctx context.Context, id string, decision RestaurantDecision) (Order, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Order{}, fmt.Errorf("begin decision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, id).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("lock order: %w", err)
	}

	targetStatus := StatusRejected
	if decision.Accepted {
		targetStatus = StatusConfirmed
	}
	if currentStatus == targetStatus {
		if err = tx.Rollback(ctx); err != nil {
			return Order{}, fmt.Errorf("rollback unchanged decision: %w", err)
		}
		return r.Get(ctx, id)
	}
	if currentStatus != StatusPending {
		return Order{}, ErrInvalidTransition
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $2, external_order_id = NULLIF($3, ''), updated_at = now()
		WHERE id = $1`, id, targetStatus, decision.ExternalOrderID)
	if err != nil {
		return Order{}, fmt.Errorf("update order decision: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, previous_status, status, source)
		VALUES ($1, $2, $3, 'restaurant')`, id, currentStatus, targetStatus)
	if err != nil {
		return Order{}, fmt.Errorf("insert decision history: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit restaurant decision: %w", err)
	}
	return r.Get(ctx, id)
}

func loadOrder(ctx context.Context, q querier, id string) (Order, error) {
	var order Order
	var externalOrderID pgtype.Text
	err := q.QueryRow(ctx, `
		SELECT o.id, o.customer_id, o.restaurant_id, o.status, o.total_price,
		       o.external_order_id, o.created_at, o.updated_at, r.integration_url
		FROM orders o
		JOIN restaurants r ON r.id = o.restaurant_id
		WHERE o.id = $1`, id).Scan(
		&order.ID, &order.CustomerID, &order.RestaurantID, &order.Status, &order.TotalPrice,
		&externalOrderID, &order.CreatedAt, &order.UpdatedAt, &order.RestaurantIntegrationURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("query order: %w", err)
	}
	if externalOrderID.Valid {
		order.ExternalOrderID = externalOrderID.String
	}

	itemRows, err := q.Query(ctx, `
		SELECT menu_item_id, external_item_id, name, unit_price, quantity
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at, id`, id)
	if err != nil {
		return Order{}, fmt.Errorf("query order items: %w", err)
	}
	defer itemRows.Close()
	order.Items = make([]OrderItem, 0)
	for itemRows.Next() {
		var item OrderItem
		if err = itemRows.Scan(&item.MenuItemID, &item.ExternalItemID, &item.Name, &item.UnitPrice, &item.Quantity); err != nil {
			return Order{}, fmt.Errorf("scan order item: %w", err)
		}
		order.Items = append(order.Items, item)
	}
	if err = itemRows.Err(); err != nil {
		return Order{}, fmt.Errorf("iterate order items: %w", err)
	}

	historyRows, err := q.Query(ctx, `
		SELECT previous_status, status, source, created_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at, id`, id)
	if err != nil {
		return Order{}, fmt.Errorf("query order history: %w", err)
	}
	defer historyRows.Close()
	order.StatusHistory = make([]StatusHistory, 0)
	for historyRows.Next() {
		var history StatusHistory
		var previousStatus pgtype.Text
		if err = historyRows.Scan(&previousStatus, &history.Status, &history.Source, &history.CreatedAt); err != nil {
			return Order{}, fmt.Errorf("scan order history: %w", err)
		}
		if previousStatus.Valid {
			history.PreviousStatus = previousStatus.String
		}
		order.StatusHistory = append(order.StatusHistory, history)
	}
	if err = historyRows.Err(); err != nil {
		return Order{}, fmt.Errorf("iterate order history: %w", err)
	}
	return order, nil
}
