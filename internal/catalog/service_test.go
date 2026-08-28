package catalog

import (
	"context"
	"errors"
	"testing"
)

const testRestaurantID = "10000000-0000-0000-0000-000000000001"

type repositoryStub struct {
	restaurants []RestaurantSummary
	restaurant  Restaurant
	categories  []MenuCategory
	err         error
}

func (r repositoryStub) ListRestaurants(context.Context) ([]RestaurantSummary, error) {
	return r.restaurants, r.err
}

func (r repositoryStub) GetRestaurant(context.Context, string) (Restaurant, error) {
	return r.restaurant, r.err
}

func (r repositoryStub) GetMenu(context.Context, string) ([]MenuCategory, error) {
	return r.categories, r.err
}

func TestGetRestaurantRejectsInvalidID(t *testing.T) {
	service := NewService(repositoryStub{})

	_, err := service.GetRestaurant(context.Background(), "not-a-uuid")

	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestGetMenuReturnsEmptyCategoriesAsArray(t *testing.T) {
	service := NewService(repositoryStub{restaurant: Restaurant{ID: testRestaurantID}})

	menu, err := service.GetMenu(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get menu: %v", err)
	}
	if menu.Categories == nil {
		t.Fatal("expected non-nil categories")
	}
	if len(menu.Categories) != 0 {
		t.Fatalf("expected empty categories, got %d", len(menu.Categories))
	}
}

func TestGetRestaurantPreservesNotFoundError(t *testing.T) {
	service := NewService(repositoryStub{err: ErrNotFound})

	_, err := service.GetRestaurant(context.Background(), testRestaurantID)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRestaurantNormalizesMissingImagesToArray(t *testing.T) {
	service := NewService(repositoryStub{restaurant: Restaurant{ID: testRestaurantID}})

	restaurant, err := service.GetRestaurant(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get restaurant: %v", err)
	}
	if restaurant.Images == nil || len(restaurant.Images) != 0 {
		t.Fatalf("expected empty images array, got %#v", restaurant.Images)
	}
}

func TestListRestaurantsNormalizesEmptyResult(t *testing.T) {
	service := NewService(repositoryStub{})
	restaurants, err := service.ListRestaurants(context.Background())
	if err != nil || restaurants == nil || len(restaurants) != 0 {
		t.Fatalf("unexpected result: %#v, %v", restaurants, err)
	}
}

func TestListRestaurantsPropagatesRepositoryError(t *testing.T) {
	service := NewService(repositoryStub{err: errors.New("query failed")})
	if _, err := service.ListRestaurants(context.Background()); err == nil {
		t.Fatal("expected repository error")
	}
}

func TestGetMenuRejectsInvalidRestaurantID(t *testing.T) {
	service := NewService(repositoryStub{})
	if _, err := service.GetMenu(context.Background(), "invalid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestGetMenuPreservesRestaurantNotFound(t *testing.T) {
	service := NewService(repositoryStub{err: ErrNotFound})
	if _, err := service.GetMenu(context.Background(), testRestaurantID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
