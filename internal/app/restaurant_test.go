package app

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestaurantCreateOrderIsIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRestaurantRouter(logger)
	body := []byte(`{
		"order_id":"40000000-0000-0000-0000-000000000001",
		"items":[{"external_item_id":"margherita","quantity":1}]
	}`)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body)))
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first status %d, got %d: %s", http.StatusCreated, first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body)))
	if second.Code != http.StatusOK {
		t.Fatalf("expected repeated status %d, got %d: %s", http.StatusOK, second.Code, second.Body.String())
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("expected identical responses, got %s and %s", first.Body.String(), second.Body.String())
	}
}

func TestRestaurantRejectsUnknownItem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRestaurantRouter(logger)
	body := []byte(`{
		"order_id":"40000000-0000-0000-0000-000000000002",
		"items":[{"external_item_id":"unknown","quantity":1}]
	}`)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body)))
	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
}
