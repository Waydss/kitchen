package catalog

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListRestaurants(ctx context.Context) ([]RestaurantSummary, error) {
	restaurants, err := s.repository.ListRestaurants(ctx)
	if err != nil {
		return nil, fmt.Errorf("list restaurants: %w", err)
	}
	if restaurants == nil {
		restaurants = make([]RestaurantSummary, 0)
	}
	return restaurants, nil
}

func (s *Service) GetRestaurant(ctx context.Context, id string) (Restaurant, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Restaurant{}, ErrInvalidID
	}
	restaurant, err := s.repository.GetRestaurant(ctx, id)
	if err != nil {
		return Restaurant{}, fmt.Errorf("get restaurant: %w", err)
	}
	if restaurant.Images == nil {
		restaurant.Images = make([]RestaurantImage, 0)
	}
	return restaurant, nil
}

func (s *Service) GetMenu(ctx context.Context, restaurantID string) (Menu, error) {
	if _, err := uuid.Parse(restaurantID); err != nil {
		return Menu{}, ErrInvalidID
	}
	if _, err := s.repository.GetRestaurant(ctx, restaurantID); err != nil {
		return Menu{}, fmt.Errorf("get restaurant for menu: %w", err)
	}
	categories, err := s.repository.GetMenu(ctx, restaurantID)
	if err != nil {
		return Menu{}, fmt.Errorf("get menu: %w", err)
	}
	if categories == nil {
		categories = make([]MenuCategory, 0)
	}
	return Menu{RestaurantID: restaurantID, Categories: categories}, nil
}
