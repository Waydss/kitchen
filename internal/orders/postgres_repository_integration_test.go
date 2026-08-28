//go:build integration

package orders

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryCreatesIdempotentOrderAndPersistsRejection(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repository := NewPostgresRepository(pool)
	command := CreateCommand{
		CustomerID:     uuid.NewString(),
		RestaurantID:   "10000000-0000-0000-0000-000000000001",
		IdempotencyKey: "integration-" + uuid.NewString(),
		Items: []CreateItem{{
			MenuItemID: "30000000-0000-0000-0000-000000000001",
			Quantity:   2,
		}},
	}

	created, wasCreated, err := repository.CreatePending(ctx, command)
	if err != nil {
		t.Fatalf("create pending order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", created.ID)
	})
	if !wasCreated || created.Status != StatusPending || created.TotalPrice != 118000 {
		t.Fatalf("unexpected created order: %#v, created=%v", created, wasCreated)
	}
	if len(created.Items) != 1 || created.Items[0].Name == "" || created.Items[0].ExternalItemID == "" {
		t.Fatalf("item snapshot was not persisted: %#v", created.Items)
	}
	if _, err = pool.Exec(ctx, `UPDATE menu_items SET price = 60000 WHERE id = $1`, command.Items[0].MenuItemID); err != nil {
		t.Fatalf("change current menu price: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE menu_items SET price = 59000 WHERE id = $1`, command.Items[0].MenuItemID)
	})
	snapshot, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get order after menu price change: %v", err)
	}
	if snapshot.TotalPrice != 118000 || snapshot.Items[0].UnitPrice != 59000 {
		t.Fatalf("order snapshot changed with menu: %#v", snapshot)
	}

	repeated, wasCreated, err := repository.CreatePending(ctx, command)
	if err != nil {
		t.Fatalf("repeat pending order: %v", err)
	}
	if wasCreated || repeated.ID != created.ID {
		t.Fatalf("idempotency violation: first=%s repeated=%s created=%v", created.ID, repeated.ID, wasCreated)
	}

	rejected, err := repository.ApplyRestaurantDecision(ctx, created.ID, RestaurantDecision{
		Accepted:        false,
		RejectionReason: "item_unavailable",
	})
	if err != nil {
		t.Fatalf("apply restaurant rejection: %v", err)
	}
	if rejected.Status != StatusRejected || rejected.RejectionReason != "item_unavailable" {
		t.Fatalf("rejection was not persisted: %#v", rejected)
	}
	if len(rejected.StatusHistory) != 2 || rejected.StatusHistory[1].PreviousStatus != StatusPending {
		t.Fatalf("unexpected status history: %#v", rejected.StatusHistory)
	}
	repeatedDecision, err := repository.ApplyRestaurantDecision(ctx, created.ID, RestaurantDecision{
		Accepted:        false,
		RejectionReason: "item_unavailable",
	})
	if err != nil || len(repeatedDecision.StatusHistory) != 2 {
		t.Fatalf("repeated decision must be idempotent: %#v, %v", repeatedDecision, err)
	}
	if _, err = repository.ApplyRestaurantDecision(ctx, created.ID, RestaurantDecision{
		Accepted:        true,
		ExternalOrderID: "different-result",
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected conflicting decision to fail, got %v", err)
	}

	stored, err := repository.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get stored order: %v", err)
	}
	if stored.Status != StatusRejected || stored.RejectionReason != "item_unavailable" {
		t.Fatalf("PostgreSQL is not source of truth: %#v", stored)
	}

	orders, err := repository.ListByCustomerID(ctx, command.CustomerID)
	if err != nil {
		t.Fatalf("list customer orders: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != created.ID || len(orders[0].Items) != 1 || len(orders[0].StatusHistory) != 2 {
		t.Fatalf("unexpected customer orders: %#v", orders)
	}
	empty, err := repository.ListByCustomerID(ctx, uuid.NewString())
	if err != nil {
		t.Fatalf("list unknown customer orders: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected empty orders array, got %#v", empty)
	}

	assertConcurrentCreateIsIdempotent(t, repository, pool)

	unavailableCommand := command
	unavailableCommand.IdempotencyKey = "unavailable-" + uuid.NewString()
	unavailableCommand.RestaurantID = "10000000-0000-0000-0000-000000000002"
	if _, _, err = repository.CreatePending(ctx, unavailableCommand); !errors.Is(err, ErrRestaurantUnavailable) {
		t.Fatalf("expected ErrRestaurantUnavailable, got %v", err)
	}

	missingItemCommand := command
	missingItemCommand.IdempotencyKey = "missing-item-" + uuid.NewString()
	missingItemCommand.Items = []CreateItem{{MenuItemID: uuid.NewString(), Quantity: 1}}
	if _, _, err = repository.CreatePending(ctx, missingItemCommand); !errors.Is(err, ErrMenuItemNotFound) {
		t.Fatalf("expected ErrMenuItemNotFound, got %v", err)
	}

	if _, err = repository.Get(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func assertConcurrentCreateIsIdempotent(t *testing.T, repository *PostgresRepository, pool *pgxpool.Pool) {
	t.Helper()
	command := CreateCommand{
		CustomerID:     uuid.NewString(),
		RestaurantID:   "10000000-0000-0000-0000-000000000001",
		IdempotencyKey: "concurrent-" + uuid.NewString(),
		Items:          []CreateItem{{MenuItemID: "30000000-0000-0000-0000-000000000001", Quantity: 1}},
	}

	const attempts = 8
	type result struct {
		order   Order
		created bool
		err     error
	}
	results := make(chan result, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			order, created, err := repository.CreatePending(context.Background(), command)
			results <- result{order: order, created: created, err: err}
		}()
	}
	group.Wait()
	close(results)

	createdCount := 0
	var orderID string
	for item := range results {
		if item.err != nil {
			t.Fatalf("concurrent create: %v", item.err)
		}
		if item.created {
			createdCount++
		}
		if orderID == "" {
			orderID = item.order.ID
		} else if item.order.ID != orderID {
			t.Fatalf("duplicate orders created: %s and %s", orderID, item.order.ID)
		}
	}
	if createdCount != 1 {
		t.Fatalf("expected exactly one creation, got %d", createdCount)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", orderID)
	})
}
