//go:build integration

package catalog

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testRestaurantWithoutImagesID = "10000000-0000-0000-0000-000000000002"
	testCoverImageURL             = "https://images.unsplash.com/photo-1579751626657-72bc17010498?auto=format&fit=crop&w=1200&q=80"
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
	restaurants, err := repository.ListRestaurants(context.Background())
	if err != nil {
		t.Fatalf("list seeded restaurants: %v", err)
	}
	if len(restaurants) != 2 {
		t.Fatalf("unexpected restaurant list: %#v", restaurants)
	}
	var foundCover, foundWithoutCover bool
	for _, item := range restaurants {
		switch item.ID {
		case testRestaurantID:
			foundCover = item.CoverImageURL != nil && *item.CoverImageURL == testCoverImageURL
		case testRestaurantWithoutImagesID:
			foundWithoutCover = item.CoverImageURL == nil
		}
	}
	if !foundCover || !foundWithoutCover {
		t.Fatalf("cover images were not read correctly: %#v", restaurants)
	}

	restaurant, err := repository.GetRestaurant(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get seeded restaurant: %v", err)
	}
	if restaurant.Name != "Demo Pizzeria" {
		t.Fatalf("unexpected restaurant: %#v", restaurant)
	}
	if len(restaurant.Images) != 2 || restaurant.Images[0].Position != 0 || restaurant.Images[1].Position != 1 {
		t.Fatalf("unexpected restaurant images: %#v", restaurant.Images)
	}

	restaurantWithoutImages, err := repository.GetRestaurant(context.Background(), testRestaurantWithoutImagesID)
	if err != nil {
		t.Fatalf("get restaurant without images: %v", err)
	}
	if restaurantWithoutImages.Images == nil || len(restaurantWithoutImages.Images) != 0 {
		t.Fatalf("expected empty images array, got %#v", restaurantWithoutImages.Images)
	}

	categories, err := repository.GetMenu(context.Background(), testRestaurantID)
	if err != nil {
		t.Fatalf("get seeded menu: %v", err)
	}
	if len(categories) != 2 || len(categories[0].Items) != 2 {
		t.Fatalf("unexpected menu: %#v", categories)
	}
	if categories[0].Items[0].ImageURL == nil || categories[0].Items[1].ImageURL == nil {
		t.Fatalf("expected pizza images, got %#v", categories[0].Items)
	}
	if len(categories[1].Items) != 1 || categories[1].Items[0].ImageURL != nil {
		t.Fatalf("expected lemonade without image, got %#v", categories[1].Items)
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin cover constraint check: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(context.Background(), `
		INSERT INTO restaurant_images (restaurant_id, url, position, is_cover)
		VALUES ($1, 'https://example.com/second-cover.jpg', 99, true)`, testRestaurantID)
	if err == nil {
		t.Fatal("expected database to reject a second cover image")
	}

	missingCoverTx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin missing cover constraint check: %v", err)
	}
	defer func() { _ = missingCoverTx.Rollback(context.Background()) }()
	_, err = missingCoverTx.Exec(context.Background(), `
		INSERT INTO restaurant_images (restaurant_id, url, position, is_cover)
		VALUES ($1, 'https://example.com/non-cover.jpg', 0, false)`, testRestaurantWithoutImagesID)
	if err != nil {
		t.Fatalf("insert deferred non-cover image: %v", err)
	}
	if err = missingCoverTx.Commit(context.Background()); err == nil {
		t.Fatal("expected database to reject an image set without a cover")
	}
}
