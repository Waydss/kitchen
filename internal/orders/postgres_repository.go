package orders

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

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
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Order{}, fmt.Errorf("begin idempotency read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM orders
		WHERE customer_id = $1 AND idempotency_key = $2`, customerID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("query order by idempotency key: %w", err)
	}
	order, err := loadOrder(ctx, tx, id)
	if err != nil {
		return Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit idempotency read transaction: %w", err)
	}
	return order, nil
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

	order, err := r.Get(ctx, orderID)
	if err != nil {
		return Order{}, false, err
	}
	order.RestaurantIntegrationURL = integrationURL
	return order, true, nil
}

func (r *PostgresRepository) Get(ctx context.Context, id string) (Order, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Order{}, fmt.Errorf("begin order read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := loadOrder(ctx, tx, id)
	if err != nil {
		return Order{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Order{}, fmt.Errorf("commit order read transaction: %w", err)
	}
	return order, nil
}

func (r *PostgresRepository) ListByCustomerID(ctx context.Context, customerID string) ([]Order, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin customer orders read transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT o.id, o.customer_id, o.restaurant_id, o.status, o.total_price,
		       o.external_order_id, o.rejection_reason, o.created_at, o.updated_at, r.integration_url
		FROM orders o
		JOIN restaurants r ON r.id = o.restaurant_id
		WHERE o.customer_id = $1
		ORDER BY o.created_at DESC, o.id`, customerID)
	if err != nil {
		return nil, fmt.Errorf("query customer orders: %w", err)
	}

	orders := make([]Order, 0)
	orderIndexes := make(map[string]int)
	for rows.Next() {
		var order Order
		var externalOrderID, rejectionReason pgtype.Text
		if err = rows.Scan(
			&order.ID, &order.CustomerID, &order.RestaurantID, &order.Status, &order.TotalPrice,
			&externalOrderID, &rejectionReason, &order.CreatedAt, &order.UpdatedAt, &order.RestaurantIntegrationURL,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan customer order: %w", err)
		}
		if externalOrderID.Valid {
			order.ExternalOrderID = externalOrderID.String
		}
		if rejectionReason.Valid {
			order.RejectionReason = rejectionReason.String
		}
		order.Items = make([]OrderItem, 0)
		order.StatusHistory = make([]StatusHistory, 0)
		orderIndexes[order.ID] = len(orders)
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate customer orders: %w", err)
	}
	rows.Close()

	if len(orders) > 0 {
		if err = loadCustomerOrderItems(ctx, tx, customerID, orders, orderIndexes); err != nil {
			return nil, err
		}
		if err = loadCustomerOrderHistory(ctx, tx, customerID, orders, orderIndexes); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit customer orders read transaction: %w", err)
	}
	return orders, nil
}

func loadCustomerOrderItems(
	ctx context.Context,
	q querier,
	customerID string,
	orders []Order,
	orderIndexes map[string]int,
) error {
	rows, err := q.Query(ctx, `
		SELECT oi.order_id, oi.menu_item_id, oi.external_item_id, oi.name, oi.unit_price, oi.quantity
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.customer_id = $1
		ORDER BY o.created_at DESC, o.id, oi.created_at, oi.id`, customerID)
	if err != nil {
		return fmt.Errorf("query customer order items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderID string
		var item OrderItem
		if err = rows.Scan(
			&orderID, &item.MenuItemID, &item.ExternalItemID, &item.Name, &item.UnitPrice, &item.Quantity,
		); err != nil {
			return fmt.Errorf("scan customer order item: %w", err)
		}
		index, exists := orderIndexes[orderID]
		if !exists {
			return fmt.Errorf("customer order item references unknown order %s", orderID)
		}
		orders[index].Items = append(orders[index].Items, item)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate customer order items: %w", err)
	}
	return nil
}

func loadCustomerOrderHistory(
	ctx context.Context,
	q querier,
	customerID string,
	orders []Order,
	orderIndexes map[string]int,
) error {
	rows, err := q.Query(ctx, `
		SELECT h.order_id, h.previous_status, h.status, h.source, h.created_at
		FROM order_status_history h
		JOIN orders o ON o.id = h.order_id
		WHERE o.customer_id = $1
		ORDER BY o.created_at DESC, o.id, h.created_at, h.id`, customerID)
	if err != nil {
		return fmt.Errorf("query customer order history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderID string
		var history StatusHistory
		var previousStatus pgtype.Text
		if err = rows.Scan(&orderID, &previousStatus, &history.Status, &history.Source, &history.CreatedAt); err != nil {
			return fmt.Errorf("scan customer order history: %w", err)
		}
		if previousStatus.Valid {
			history.PreviousStatus = previousStatus.String
		}
		index, exists := orderIndexes[orderID]
		if !exists {
			return fmt.Errorf("customer order history references unknown order %s", orderID)
		}
		orders[index].StatusHistory = append(orders[index].StatusHistory, history)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate customer order history: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ApplyRestaurantDecision(ctx context.Context, id string, decision RestaurantDecision) (Order, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Order{}, fmt.Errorf("begin decision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus string
	var currentExternalOrderID, currentRejectionReason pgtype.Text
	err = tx.QueryRow(ctx, `
		SELECT status, external_order_id, rejection_reason
		FROM orders
		WHERE id = $1
		FOR UPDATE`, id).Scan(&currentStatus, &currentExternalOrderID, &currentRejectionReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("lock order: %w", err)
	}

	targetStatus := StatusRejected
	if decision.Accepted {
		if strings.TrimSpace(decision.ExternalOrderID) == "" {
			return Order{}, ErrInvalidTransition
		}
		targetStatus = StatusConfirmed
		decision.RejectionReason = ""
	} else if decision.RejectionReason == "" {
		decision.RejectionReason = "restaurant_rejected"
	}
	if currentStatus == targetStatus {
		if (targetStatus == StatusConfirmed && currentExternalOrderID.String != decision.ExternalOrderID) ||
			(targetStatus == StatusRejected && currentRejectionReason.String != decision.RejectionReason) {
			return Order{}, ErrInvalidTransition
		}
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
		SET status = $2,
		    external_order_id = NULLIF($3, ''),
		    rejection_reason = NULLIF($4, ''),
		    updated_at = now()
		WHERE id = $1`, id, targetStatus, decision.ExternalOrderID, decision.RejectionReason)
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
	var externalOrderID, rejectionReason pgtype.Text
	err := q.QueryRow(ctx, `
		SELECT o.id, o.customer_id, o.restaurant_id, o.status, o.total_price,
		       o.external_order_id, o.rejection_reason, o.created_at, o.updated_at, r.integration_url
		FROM orders o
		JOIN restaurants r ON r.id = o.restaurant_id
		WHERE o.id = $1`, id).Scan(
		&order.ID, &order.CustomerID, &order.RestaurantID, &order.Status, &order.TotalPrice,
		&externalOrderID, &rejectionReason, &order.CreatedAt, &order.UpdatedAt, &order.RestaurantIntegrationURL,
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
	if rejectionReason.Valid {
		order.RejectionReason = rejectionReason.String
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
