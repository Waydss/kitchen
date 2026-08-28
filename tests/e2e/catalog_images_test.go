//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

const restaurantWithoutImagesID = "10000000-0000-0000-0000-000000000002"

type restaurantSummaryResponse struct {
	ID            string  `json:"id"`
	CoverImageURL *string `json:"cover_image_url"`
}

type restaurantImageResponse struct {
	URL      string `json:"url"`
	Position int    `json:"position"`
}

type restaurantDetailsResponse struct {
	ID     string                    `json:"id"`
	Images []restaurantImageResponse `json:"images"`
}

type menuResponse struct {
	Categories []struct {
		Items []struct {
			ID       string  `json:"id"`
			ImageURL *string `json:"image_url"`
		} `json:"items"`
	} `json:"categories"`
}

func TestCatalogImageResponses(t *testing.T) {
	apiURL := os.Getenv("E2E_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 5 * time.Second}

	var list struct {
		Restaurants []restaurantSummaryResponse `json:"restaurants"`
	}
	getJSON(t, client, apiURL+"/api/v1/restaurants", &list)
	if len(list.Restaurants) != 2 {
		t.Fatalf("expected two restaurants, got %#v", list.Restaurants)
	}
	assertRestaurantCovers(t, list.Restaurants)

	var details restaurantDetailsResponse
	getJSON(t, client, apiURL+"/api/v1/restaurants/"+demoRestaurantID, &details)
	if len(details.Images) != 2 || details.Images[0].Position != 0 || details.Images[1].Position != 1 {
		t.Fatalf("restaurant images are missing or unordered: %#v", details.Images)
	}

	var withoutImages restaurantDetailsResponse
	getJSON(t, client, apiURL+"/api/v1/restaurants/"+restaurantWithoutImagesID, &withoutImages)
	if withoutImages.Images == nil || len(withoutImages.Images) != 0 {
		t.Fatalf("expected an explicit empty images array, got %#v", withoutImages.Images)
	}

	var menu menuResponse
	getJSON(t, client, apiURL+"/api/v1/restaurants/"+demoRestaurantID+"/menu", &menu)
	if len(menu.Categories) != 2 || len(menu.Categories[0].Items) != 2 || len(menu.Categories[1].Items) != 1 {
		t.Fatalf("unexpected menu shape: %#v", menu)
	}
	if menu.Categories[0].Items[0].ImageURL == nil || menu.Categories[0].Items[1].ImageURL == nil {
		t.Fatalf("expected seeded pizza image URLs: %#v", menu.Categories[0].Items)
	}
	if menu.Categories[1].Items[0].ImageURL != nil {
		t.Fatalf("expected explicit null for lemonade image: %#v", menu.Categories[1].Items[0])
	}
}

func assertRestaurantCovers(t *testing.T, restaurants []restaurantSummaryResponse) {
	t.Helper()
	var foundCover, foundWithoutCover bool
	for _, restaurant := range restaurants {
		switch restaurant.ID {
		case demoRestaurantID:
			foundCover = restaurant.CoverImageURL != nil && *restaurant.CoverImageURL != ""
		case restaurantWithoutImagesID:
			foundWithoutCover = restaurant.CoverImageURL == nil
		}
	}
	if !foundCover || !foundWithoutCover {
		t.Fatalf("unexpected cover image values: %#v", restaurants)
	}
}

func getJSON(t *testing.T, client *http.Client, url string, target any) {
	t.Helper()
	response, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: expected status 200, got %d", url, response.StatusCode)
	}
	if err = json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode %s response: %v", url, err)
	}
}
