package catalog

import (
	"context"
	"errors"
	"testing"
)

const testRestaurantID = "10000000-0000-0000-0000-000000000001"

type repositoryStub struct {
	restaurants []Restaurant
	restaurant  Restaurant
	categories  []MenuCategory
	err         error
}

func (r repositoryStub) ListRestaurants(context.Context) ([]Restaurant, error) {
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
