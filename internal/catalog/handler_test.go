package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type serviceStub struct {
	restaurants []Restaurant
	restaurant  Restaurant
	menu        Menu
	err         error
}

func (s serviceStub) ListRestaurants(context.Context) ([]Restaurant, error) {
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

func TestListRestaurantsReturnsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(serviceStub{restaurants: []Restaurant{{ID: testRestaurantID, Name: "Demo"}}})
	handler.RegisterRoutes(router.Group("/api/v1"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !strings.Contains(response.Body.String(), testRestaurantID) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}
