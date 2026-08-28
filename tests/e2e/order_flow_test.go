//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	demoRestaurantID = "10000000-0000-0000-0000-000000000001"
	demoMenuItemID   = "30000000-0000-0000-0000-000000000001"
)

type orderResponse struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	TotalPrice      int64  `json:"total_price"`
	ExternalOrderID string `json:"external_order_id"`
}

type errorResponse struct {
	Code string `json:"code"`
}

type ordersResponse struct {
	Orders []orderResponse `json:"orders"`
}

func TestClientOrderFlowThroughKitchenAndRestaurant(t *testing.T) {
	apiURL := os.Getenv("E2E_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	customerID := uuid.NewString()
	idempotencyKey := "e2e-" + uuid.NewString()
	payload := []byte(fmt.Sprintf(`{
		"customer_id":%q,
		"restaurant_id":%q,
		"items":[{"menu_item_id":%q,"quantity":1}]
	}`, customerID, demoRestaurantID, demoMenuItemID))

	created := submitOrder(t, client, apiURL, idempotencyKey, payload, http.StatusCreated)
	if created.Status != "confirmed" || created.TotalPrice != 59000 || created.ExternalOrderID == "" {
		t.Fatalf("Kitchen did not persist restaurant decision: %#v", created)
	}

	repeated := submitOrder(t, client, apiURL, idempotencyKey, payload, http.StatusOK)
	if repeated.ID != created.ID || repeated.ExternalOrderID != created.ExternalOrderID {
		t.Fatalf("idempotency violation: created=%#v repeated=%#v", created, repeated)
	}
	differentPayload := []byte(fmt.Sprintf(`{
		"customer_id":%q,
		"restaurant_id":%q,
		"items":[{"menu_item_id":%q,"quantity":2}]
	}`, customerID, demoRestaurantID, demoMenuItemID))
	conflict := submitOrderError(t, client, apiURL, idempotencyKey, differentPayload, http.StatusConflict)
	if conflict.Code != "idempotency_conflict" {
		t.Fatalf("unexpected idempotency conflict: %#v", conflict)
	}

	response, err := client.Get(apiURL + "/api/v1/orders/" + created.ID)
	if err != nil {
		t.Fatalf("get Kitchen order: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected GET status 200, got %d", response.StatusCode)
	}
	var stored orderResponse
	if err = json.NewDecoder(response.Body).Decode(&stored); err != nil {
		t.Fatalf("decode stored order: %v", err)
	}
	if stored.ID != created.ID || stored.Status != "confirmed" || stored.ExternalOrderID != created.ExternalOrderID {
		t.Fatalf("Kitchen PostgreSQL is not source of truth: %#v", stored)
	}

	listResponse, err := client.Get(apiURL + "/api/v1/orders?customer_id=" + customerID)
	if err != nil {
		t.Fatalf("list Kitchen orders: %v", err)
	}
	defer func() { _ = listResponse.Body.Close() }()
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listResponse.StatusCode)
	}
	var listed ordersResponse
	if err = json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatalf("decode customer orders: %v", err)
	}
	if len(listed.Orders) != 1 || listed.Orders[0].ID != created.ID || listed.Orders[0].Status != "confirmed" {
		t.Fatalf("unexpected customer orders: %#v", listed.Orders)
	}

	emptyResponse, err := client.Get(apiURL + "/api/v1/orders?customer_id=" + uuid.NewString())
	if err != nil {
		t.Fatalf("list unknown customer orders: %v", err)
	}
	defer func() { _ = emptyResponse.Body.Close() }()
	var empty ordersResponse
	if emptyResponse.StatusCode != http.StatusOK {
		t.Fatalf("expected empty list status 200, got %d", emptyResponse.StatusCode)
	}
	if err = json.NewDecoder(emptyResponse.Body).Decode(&empty); err != nil {
		t.Fatalf("decode empty customer orders: %v", err)
	}
	if empty.Orders == nil || len(empty.Orders) != 0 {
		t.Fatalf("expected empty orders array, got %#v", empty.Orders)
	}
}

func submitOrder(t *testing.T, client *http.Client, apiURL, key string, payload []byte, expectedStatus int) orderResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/orders", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("submit Kitchen order: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != expectedStatus {
		t.Fatalf("expected POST status %d, got %d", expectedStatus, response.StatusCode)
	}
	var order orderResponse
	if err = json.NewDecoder(response.Body).Decode(&order); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	return order
}

func submitOrderError(t *testing.T, client *http.Client, apiURL, key string, payload []byte, expectedStatus int) errorResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, apiURL+"/api/v1/orders", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("submit conflicting Kitchen order: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != expectedStatus {
		t.Fatalf("expected POST status %d, got %d", expectedStatus, response.StatusCode)
	}
	var responseError errorResponse
	if err = json.NewDecoder(response.Body).Decode(&responseError); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return responseError
}
