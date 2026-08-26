//go:build integration

package catalog

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryReadsSeededCatalog(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	repository := NewPostgresRepository(pool)
	restaurant, err := repository.GetRestaurant(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get seeded restaurant: %v", err)
	}
	if restaurant.Name != "Demo Pizzeria" {
		t.Fatalf("unexpected restaurant: %#v", restaurant)
	}

	categories, err := repository.GetMenu(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get seeded menu: %v", err)
	}
	if len(categories) != 2 || len(categories[0].Items) != 2 {
		t.Fatalf("unexpected menu: %#v", categories)
	}
}
