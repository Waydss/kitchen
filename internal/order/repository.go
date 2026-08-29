package order

import "context"

type Repository interface {
	FindByIdempotencyKey(ctx context.Context, customerID, key string) (Order, error)
	CreatePending(ctx context.Context, command CreateCommand) (order Order, created bool, err error)
	ListByCustomerID(ctx context.Context, customerID string) ([]Order, error)
	Get(ctx context.Context, id string) (Order, error)
	ApplyRestaurantDecision(ctx context.Context, id string, decision RestaurantDecision) (Order, error)
}

type RestaurantClient interface {
	SubmitOrder(ctx context.Context, order Order) (RestaurantDecision, error)
}
