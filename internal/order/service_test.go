package order

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
	existing      Order
	orders        []Order
	createResult  Order
	updated       Order
	findErr       error
	listErr       error
	createErr     error
	updateErr     error
	forceExisting bool
	createCalls   int
	updateCalls   int
	listCalls     int
}

func (r *repositoryStub) FindByIdempotencyKey(context.Context, string, string) (Order, error) {
	return r.existing, r.findErr
}

func (r *repositoryStub) CreatePending(context.Context, CreateCommand) (Order, bool, error) {
	r.createCalls++
	return r.createResult, !r.forceExisting, r.createErr
}

func (r *repositoryStub) Get(context.Context, string) (Order, error) {
	return r.existing, r.findErr
}

func (r *repositoryStub) ListByCustomerID(context.Context, string) ([]Order, error) {
	r.listCalls++
	return r.orders, r.listErr
}

func (r *repositoryStub) ApplyRestaurantDecision(context.Context, string, RestaurantDecision) (Order, error) {
	r.updateCalls++
	return r.updated, r.updateErr
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

func TestInvalidPartnerResponseIsNotReportedAsTemporaryUnavailability(t *testing.T) {
	pending := testOrder(StatusPending)
	repository := &repositoryStub{findErr: ErrNotFound, createResult: pending}
	client := &restaurantClientStub{err: ErrInvalidPartnerResponse}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if !errors.Is(err, ErrInvalidPartnerResponse) {
		t.Fatalf("expected ErrInvalidPartnerResponse, got %v", err)
	}
	if result.Order.ID != "" || repository.updateCalls != 0 {
		t.Fatalf("invalid partner response must not be reported as a retryable result: %#v", result)
	}
}

func TestCreateOrderPersistsRestaurantRejection(t *testing.T) {
	repository := &repositoryStub{
		findErr:      ErrNotFound,
		createResult: testOrder(StatusPending),
		updated:      testOrder(StatusRejected),
	}
	client := &restaurantClientStub{decision: RestaurantDecision{Accepted: false, RejectionReason: "item_unavailable"}}
	service := NewService(repository, client)

	result, err := service.Create(context.Background(), testCommand())
	if err != nil {
		t.Fatalf("create rejected order: %v", err)
	}
	if result.Order.Status != StatusRejected || repository.updateCalls != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCreateOrderPropagatesRepositoryFailures(t *testing.T) {
	tests := []struct {
		name       string
		repository *repositoryStub
		client     *restaurantClientStub
	}{
		{name: "idempotency lookup", repository: &repositoryStub{findErr: errors.New("lookup failed")}, client: &restaurantClientStub{}},
		{name: "create pending", repository: &repositoryStub{findErr: ErrNotFound, createErr: errors.New("insert failed")}, client: &restaurantClientStub{}},
		{name: "apply decision", repository: &repositoryStub{findErr: ErrNotFound, createResult: testOrder(StatusPending), updateErr: errors.New("update failed")}, client: &restaurantClientStub{decision: RestaurantDecision{Accepted: true, ExternalOrderID: "external-1"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(test.repository, test.client)
			if _, err := service.Create(context.Background(), testCommand()); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestConcurrentConflictResultRejectsDifferentPayload(t *testing.T) {
	existing := testOrder(StatusPending)
	existing.Items[0].Quantity = 2
	repository := &repositoryStub{findErr: ErrNotFound, createResult: existing, forceExisting: true}
	service := NewService(repository, &restaurantClientStub{})

	_, err := service.Create(context.Background(), testCommand())
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestValidateCreateCommandRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CreateCommand)
	}{
		{name: "customer id", mutate: func(command *CreateCommand) { command.CustomerID = "invalid" }},
		{name: "restaurant id", mutate: func(command *CreateCommand) { command.RestaurantID = "invalid" }},
		{name: "short idempotency key", mutate: func(command *CreateCommand) { command.IdempotencyKey = "short" }},
		{name: "long idempotency key", mutate: func(command *CreateCommand) { command.IdempotencyKey = string(make([]byte, 129)) }},
		{name: "empty items", mutate: func(command *CreateCommand) { command.Items = nil }},
		{name: "too many items", mutate: func(command *CreateCommand) {
			command.Items = make([]CreateItem, 51)
			for index := range command.Items {
				command.Items[index] = CreateItem{MenuItemID: testMenuItemID, Quantity: 1}
			}
		}},
		{name: "invalid item id", mutate: func(command *CreateCommand) { command.Items[0].MenuItemID = "invalid" }},
		{name: "zero quantity", mutate: func(command *CreateCommand) { command.Items[0].Quantity = 0 }},
		{name: "large quantity", mutate: func(command *CreateCommand) { command.Items[0].Quantity = 21 }},
		{name: "duplicate item", mutate: func(command *CreateCommand) { command.Items = append(command.Items, command.Items[0]) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := testCommand()
			test.mutate(&command)
			if err := validateCreateCommand(command); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected ErrInvalidRequest, got %v", err)
			}
		})
	}
}

func TestGetOrderValidatesAndReturnsRepositoryResult(t *testing.T) {
	service := NewService(&repositoryStub{existing: testOrder(StatusConfirmed)}, &restaurantClientStub{})
	order, err := service.Get(context.Background(), testOrderID)
	if err != nil || order.ID != testOrderID {
		t.Fatalf("unexpected result: %#v, %v", order, err)
	}
	if _, err = service.Get(context.Background(), "invalid"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestListOrdersNormalizesNilAndPropagatesError(t *testing.T) {
	service := NewService(&repositoryStub{}, &restaurantClientStub{})
	orders, err := service.ListByCustomerID(context.Background(), testCustomerID)
	if err != nil || orders == nil || len(orders) != 0 {
		t.Fatalf("unexpected empty result: %#v, %v", orders, err)
	}

	service = NewService(&repositoryStub{listErr: errors.New("query failed")}, &restaurantClientStub{})
	if _, err = service.ListByCustomerID(context.Background(), testCustomerID); err == nil {
		t.Fatal("expected repository error")
	}
}

func TestServiceListOrdersReturnsCustomerOrders(t *testing.T) {
	repository := &repositoryStub{orders: []Order{testOrder(StatusConfirmed)}}
	service := NewService(repository, &restaurantClientStub{})

	orders, err := service.ListByCustomerID(context.Background(), testCustomerID)
	if err != nil {
		t.Fatalf("list customer orders: %v", err)
	}
	if repository.listCalls != 1 || len(orders) != 1 || orders[0].ID != testOrderID {
		t.Fatalf("unexpected orders: %#v, calls=%d", orders, repository.listCalls)
	}
}

func TestServiceListOrdersRejectsInvalidCustomerID(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, &restaurantClientStub{})

	_, err := service.ListByCustomerID(context.Background(), "invalid")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
	if repository.listCalls != 0 {
		t.Fatalf("repository must not be called, got %d calls", repository.listCalls)
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
