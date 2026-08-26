package orders

import (
	"context"
	"errors"
	"testing"
)

const (
	testCustomerID   = "50000000-0000-0000-0000-000000000001"
	testRestaurantID = "10000000-0000-0000-0000-000000000001"
	testMenuItemID   = "30000000-0000-0000-0000-000000000001"
	testOrderID      = "40000000-0000-0000-0000-000000000001"
)

type repositoryStub struct {
	existing     Order
	createResult Order
	updated      Order
	findErr      error
	createCalls  int
	updateCalls  int
}

func (r *repositoryStub) FindByIdempotencyKey(context.Context, string, string) (Order, error) {
	return r.existing, r.findErr
}

func (r *repositoryStub) CreatePending(context.Context, CreateCommand) (Order, bool, error) {
	r.createCalls++
	return r.createResult, true, nil
}

func (r *repositoryStub) Get(context.Context, string) (Order, error) {
	return r.existing, r.findErr
}

func (r *repositoryStub) ApplyRestaurantDecision(context.Context, string, RestaurantDecision) (Order, error) {
	r.updateCalls++
	return r.updated, nil
}

type restaurantClientStub struct {
	decision RestaurantDecision
	err      error
	calls    int
}

func (c *restaurantClientStub) SubmitOrder(context.Context, Order) (RestaurantDecision, error) {
	c.calls++
	return c.decision, c.err
}

func TestCreateOrderPersistsRestaurantDecision(t *testing.T) {
	pending := testOrder(StatusPending)
	confirmed := testOrder(StatusConfirmed)
	repository := &repositoryStub{findErr: ErrNotFound, createResult: pending, updated: confirmed}
	client := &restaurantClientStub{decision: RestaurantDecision{Accepted: true, ExternalOrderID: "external-1"}}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if result.Order.Status != StatusConfirmed || !result.Created {
		t.Fatalf("unexpected result: %#v", result)
	}
	if repository.createCalls != 1 || repository.updateCalls != 1 || client.calls != 1 {
		t.Fatalf("unexpected calls: create=%d update=%d client=%d", repository.createCalls, repository.updateCalls, client.calls)
	}
}

func TestRepeatedConfirmedOrderDoesNotCallRestaurant(t *testing.T) {
	repository := &repositoryStub{existing: testOrder(StatusConfirmed)}
	client := &restaurantClientStub{}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if err != nil {
		t.Fatalf("repeat order: %v", err)
	}
	if result.Created || result.Order.ID != testOrderID {
		t.Fatalf("unexpected result: %#v", result)
	}
	if repository.createCalls != 0 || client.calls != 0 {
		t.Fatalf("unexpected repeat calls: create=%d client=%d", repository.createCalls, client.calls)
	}
}

func TestRepeatedPendingOrderRetriesSameRestaurantOrder(t *testing.T) {
	repository := &repositoryStub{existing: testOrder(StatusPending), updated: testOrder(StatusConfirmed)}
	client := &restaurantClientStub{decision: RestaurantDecision{Accepted: true, ExternalOrderID: "external-1"}}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if err != nil {
		t.Fatalf("retry pending order: %v", err)
	}
	if result.Created || result.Order.Status != StatusConfirmed || client.calls != 1 {
		t.Fatalf("unexpected retry result: %#v, calls=%d", result, client.calls)
	}
}

func TestIdempotencyKeyRejectsDifferentPayload(t *testing.T) {
	repository := &repositoryStub{existing: testOrder(StatusConfirmed)}
	service := NewService(repository, &restaurantClientStub{})
	command := testCommand()
	command.Items[0].Quantity = 2

	_, err := service.Create(context.Background(), command)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestPartnerFailureKeepsOrderPending(t *testing.T) {
	pending := testOrder(StatusPending)
	repository := &repositoryStub{findErr: ErrNotFound, createResult: pending}
	client := &restaurantClientStub{err: ErrPartnerUnavailable}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if !errors.Is(err, ErrPartnerUnavailable) {
		t.Fatalf("expected ErrPartnerUnavailable, got %v", err)
	}
	if !result.Pending || result.Order.Status != StatusPending || repository.updateCalls != 0 {
		t.Fatalf("unexpected pending result: %#v", result)
	}
}

func testCommand() CreateCommand {
	return CreateCommand{
		CustomerID:     testCustomerID,
		RestaurantID:   testRestaurantID,
		IdempotencyKey: "test-key-0001",
		Items:          []CreateItem{{MenuItemID: testMenuItemID, Quantity: 1}},
	}
}

func testOrder(status string) Order {
	return Order{
		ID:           testOrderID,
		CustomerID:   testCustomerID,
		RestaurantID: testRestaurantID,
		Status:       status,
		Items:        []OrderItem{{MenuItemID: testMenuItemID, Quantity: 1}},
	}
}
