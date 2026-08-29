package restaurant

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInfoReturnsRestaurantIdentity(t *testing.T) {
	router := restaurantTestRouter(NewHandler())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/info", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"external_restaurant_id":"demo-pizzeria"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateOrderIsIdempotentAndDecrementsStockOnce(t *testing.T) {
	handler := NewHandler()
	router := restaurantTestRouter(handler)
	body := `{"order_id":"40000000-0000-0000-0000-000000000001","items":[{"external_item_id":"margherita","quantity":2}]}`

	first := performRestaurantOrder(router, body)
	second := performRestaurantOrder(router, body)
	if first.Code != http.StatusCreated || second.Code != http.StatusOK || first.Body.String() != second.Body.String() {
		t.Fatalf("unexpected responses: first=%d %s second=%d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	if handler.stock["margherita"] != 48 {
		t.Fatalf("stock was decremented more than once: %d", handler.stock["margherita"])
	}
}

func TestConcurrentRepeatedOrderIsCreatedOnce(t *testing.T) {
	handler := NewHandler()
	router := restaurantTestRouter(handler)
	body := `{"order_id":"40000000-0000-0000-0000-000000000002","items":[{"external_item_id":"pepperoni","quantity":3}]}`

	const requests = 12
	responses := make(chan *httptest.ResponseRecorder, requests)
	var group sync.WaitGroup
	for range requests {
		group.Add(1)
		go func() {
			defer group.Done()
			responses <- performRestaurantOrder(router, body)
		}()
	}
	group.Wait()
	close(responses)

	created := 0
	var externalOrderID string
	for response := range responses {
		if response.Code == http.StatusCreated {
			created++
		} else if response.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
		}
		var result orderResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if externalOrderID == "" {
			externalOrderID = result.ExternalOrderID
		} else if result.ExternalOrderID != externalOrderID {
			t.Fatalf("non-deterministic external order ids: %s and %s", externalOrderID, result.ExternalOrderID)
		}
	}
	if created != 1 || handler.stock["pepperoni"] != 37 {
		t.Fatalf("expected one creation and stock 37, got creations=%d stock=%d", created, handler.stock["pepperoni"])
	}
}

func TestUnavailableOrderReturnsStableRejection(t *testing.T) {
	handler := NewHandler()
	router := restaurantTestRouter(handler)
	body := `{"order_id":"40000000-0000-0000-0000-000000000003","items":[{"external_item_id":"unknown","quantity":1}]}`

	first := performRestaurantOrder(router, body)
	second := performRestaurantOrder(router, body)
	if first.Code != http.StatusConflict || second.Code != http.StatusConflict || first.Body.String() != second.Body.String() {
		t.Fatalf("rejection is not stable: first=%d %s second=%d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
}

func TestRepeatedOrderWithDifferentPayloadReturnsConflict(t *testing.T) {
	router := restaurantTestRouter(NewHandler())
	first := `{"order_id":"40000000-0000-0000-0000-000000000004","items":[{"external_item_id":"lemonade","quantity":1}]}`
	second := `{"order_id":"40000000-0000-0000-0000-000000000004","items":[{"external_item_id":"lemonade","quantity":2}]}`

	if response := performRestaurantOrder(router, first); response.Code != http.StatusCreated {
		t.Fatalf("unexpected first response: %d %s", response.Code, response.Body.String())
	}
	response := performRestaurantOrder(router, second)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"idempotency_conflict"`) {
		t.Fatalf("unexpected conflict: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateOrderRejectsInvalidInput(t *testing.T) {
	router := restaurantTestRouter(NewHandler())
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
	}{
		{name: "duplicate item", body: `{"order_id":"40000000-0000-0000-0000-000000000005","items":[{"external_item_id":"lemonade","quantity":1},{"external_item_id":"lemonade","quantity":1}]}`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "blank item", body: `{"order_id":"40000000-0000-0000-0000-000000000005","items":[{"external_item_id":" ","quantity":1}]}`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "unknown field", body: `{"order_id":"40000000-0000-0000-0000-000000000005","items":[],"extra":true}`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "oversized", body: `{"padding":"` + strings.Repeat("x", 1<<20) + `"}`, contentType: "application/json", status: http.StatusRequestEntityTooLarge},
		{name: "wrong content type", body: `{}`, contentType: "text/plain", status: http.StatusUnsupportedMediaType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
		})
	}
}

func restaurantTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}

func performRestaurantOrder(router http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
