//go:build integration

package orders

import (
	"context"
	"os"
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
}
