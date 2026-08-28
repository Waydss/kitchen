package catalog

import "context"

type Repository interface {
	ListRestaurants(ctx context.Context) ([]RestaurantSummary, error)
	GetRestaurant(ctx context.Context, id string) (Restaurant, error)
	GetMenu(ctx context.Context, restaurantID string) ([]MenuCategory, error)
}
