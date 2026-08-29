package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/order"
)

type restaurantClientStub struct{}

func (restaurantClientStub) SubmitOrder(context.Context, order.Order) (order.RestaurantDecision, error) {
	return order.RestaurantDecision{}, nil
}

func TestKitchenRouterInfrastructureEndpoints(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://invalid:invalid@127.0.0.1:1/invalid?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("create unavailable pool: %v", err)
	}
	t.Cleanup(pool.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewKitchenRouter(pool, restaurantClientStub{}, "http://localhost:8082", logger)

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "liveness", method: http.MethodGet, path: "/health/live", status: http.StatusOK},
		{name: "readiness without database", method: http.MethodGet, path: "/health/ready", status: http.StatusServiceUnavailable},
		{name: "unknown route", method: http.MethodGet, path: "/unknown", status: http.StatusNotFound},
		{name: "unsupported method", method: http.MethodDelete, path: "/health/live", status: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("expected X-Request-ID response header")
			}
		})
	}
}
