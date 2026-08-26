package catalog

import "context"

type Repository interface {
	ListRestaurants(ctx context.Context) ([]Restaurant, error)
	GetRestaurant(ctx context.Context, id string) (Restaurant, error)
	GetMenu(ctx context.Context, restaurantID string) ([]MenuCategory, error)
}
