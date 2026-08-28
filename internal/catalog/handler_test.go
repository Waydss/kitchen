package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type serviceStub struct {
	restaurants []RestaurantSummary
	restaurant  Restaurant
	menu        Menu
	err         error
}

func (s serviceStub) ListRestaurants(context.Context) ([]RestaurantSummary, error) {
	return s.restaurants, s.err
}

func (s serviceStub) GetRestaurant(context.Context, string) (Restaurant, error) {
	return s.restaurant, s.err
}

func (s serviceStub) GetMenu(context.Context, string) (Menu, error) {
	return s.menu, s.err
}

func TestGetRestaurantMapsInvalidIDToBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(serviceStub{err: ErrInvalidID})
	handler.RegisterRoutes(router.Group("/api/v1"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/invalid", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"invalid_argument"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestListRestaurantsReturnsCoverImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	coverURL := "https://example.com/restaurant/cover.jpg"
	handler := NewHandler(serviceStub{restaurants: []RestaurantSummary{{
		ID:            testRestaurantID,
		Name:          "Demo",
		CoverImageURL: &coverURL,
	}}})
	handler.RegisterRoutes(router.Group("/api/v1"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var body struct {
		Restaurants []RestaurantSummary `json:"restaurants"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Restaurants) != 1 || body.Restaurants[0].CoverImageURL == nil || *body.Restaurants[0].CoverImageURL != coverURL {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestGetRestaurantReturnsOrderedImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(serviceStub{restaurant: Restaurant{
		ID: testRestaurantID,
		Images: []RestaurantImage{
			{URL: "https://example.com/restaurant/cover.jpg", Position: 0},
			{URL: "https://example.com/restaurant/interior.jpg", Position: 1},
		},
	}})
	handler.RegisterRoutes(router.Group("/api/v1"))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+testRestaurantID, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var restaurant Restaurant
	if err := json.Unmarshal(response.Body.Bytes(), &restaurant); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(restaurant.Images) != 2 || restaurant.Images[0].Position != 0 || restaurant.Images[1].Position != 1 {
		t.Fatalf("unexpected images: %#v", restaurant.Images)
	}
}

func TestGetRestaurantWithoutImagesReturnsEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(serviceStub{restaurant: Restaurant{ID: testRestaurantID, Images: []RestaurantImage{}}})
	handler.RegisterRoutes(router.Group("/api/v1"))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+testRestaurantID, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !strings.Contains(response.Body.String(), `"images":[]`) {
		t.Fatalf("expected empty images array, got %s", response.Body.String())
	}
}

func TestGetMenuReturnsNullableImageURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	imageURL := "https://example.com/menu/pizza.jpg"
	handler := NewHandler(serviceStub{menu: Menu{
		RestaurantID: testRestaurantID,
		Categories: []MenuCategory{{
			ID: "20000000-0000-0000-0000-000000000001",
			Items: []MenuItem{
				{ID: "30000000-0000-0000-0000-000000000001", ImageURL: &imageURL},
				{ID: "30000000-0000-0000-0000-000000000002", ImageURL: nil},
			},
		}},
	}})
	handler.RegisterRoutes(router.Group("/api/v1"))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+testRestaurantID+"/menu", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	var menu Menu
	if err := json.Unmarshal(response.Body.Bytes(), &menu); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(menu.Categories) != 1 || len(menu.Categories[0].Items) != 2 {
		t.Fatalf("unexpected menu: %#v", menu)
	}
	items := menu.Categories[0].Items
	if items[0].ImageURL == nil || *items[0].ImageURL != imageURL || items[1].ImageURL != nil {
		t.Fatalf("unexpected image URLs: %#v", items)
	}
}
