package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type orderServiceStub struct {
	result     CreateResult
	order      Order
	orders     []Order
	err        error
	command    CreateCommand
	customerID string
}

func (s *orderServiceStub) Create(_ context.Context, command CreateCommand) (CreateResult, error) {
	s.command = command
	return s.result, s.err
}

func (s *orderServiceStub) Get(context.Context, string) (Order, error) {
	return s.order, s.err
}

func (s *orderServiceStub) ListByCustomerID(_ context.Context, customerID string) ([]Order, error) {
	s.customerID = customerID
	return s.orders, s.err
}

func TestCreateOrderMapsRequestAndReturnsCreated(t *testing.T) {
	service := &orderServiceStub{result: CreateResult{Order: testOrder(StatusConfirmed), Created: true}}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(`{
		"customer_id":"50000000-0000-0000-0000-000000000001",
		"restaurant_id":"10000000-0000-0000-0000-000000000001",
		"items":[{"menu_item_id":"30000000-0000-0000-0000-000000000001","quantity":2}]
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "handler-test-key")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if service.command.IdempotencyKey != "handler-test-key" || len(service.command.Items) != 1 || service.command.Items[0].Quantity != 2 {
		t.Fatalf("unexpected command: %#v", service.command)
	}
}

func TestCreateOrderReturnsAcceptedWhenPartnerIsUnavailable(t *testing.T) {
	service := &orderServiceStub{
		result: CreateResult{Order: testOrder(StatusPending), Created: true, Pending: true},
		err:    ErrPartnerUnavailable,
	}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(validCreateOrderJSON()))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "handler-test-key")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
}

func TestCreateOrderReturnsOKForIdempotentRepeat(t *testing.T) {
	service := &orderServiceStub{result: CreateResult{Order: testOrder(StatusConfirmed), Created: false}}
	router := testOrderRouter(service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(validCreateOrderJSON()))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "handler-test-key")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestCreateOrderRejectsInvalidHTTPPayload(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
	}{
		{name: "malformed JSON", body: `{"customer_id":`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "unknown field", body: `{"customer_id":"50000000-0000-0000-0000-000000000001","restaurant_id":"10000000-0000-0000-0000-000000000001","items":[],"extra":true}`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "trailing object", body: validCreateOrderJSON() + `{}`, contentType: "application/json", status: http.StatusBadRequest},
		{name: "oversized", body: `{"padding":"` + strings.Repeat("x", 1<<20) + `"}`, contentType: "application/json", status: http.StatusRequestEntityTooLarge},
		{name: "wrong content type", body: validCreateOrderJSON(), contentType: "text/plain", status: http.StatusUnsupportedMediaType},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &orderServiceStub{}
			router := testOrderRouter(service)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set("Idempotency-Key", "handler-test-key")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d: %s", test.status, response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateOrderMapsIdempotencyConflict(t *testing.T) {
	service := &orderServiceStub{err: ErrIdempotencyConflict}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(validCreateOrderJSON()))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "handler-test-key")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "idempotency_conflict" {
		t.Fatalf("unexpected error: %#v", body)
	}
}

func TestWriteErrorDoesNotExposeInternalDetails(t *testing.T) {
	service := &orderServiceStub{err: errors.New("database password leaked")}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders/40000000-0000-0000-0000-000000000001", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError || bytes.Contains(response.Body.Bytes(), []byte("password")) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestOrderHandlerMapsDomainErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: ErrInvalidRequest, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "restaurant not found", err: ErrRestaurantNotFound, status: http.StatusNotFound, code: "restaurant_not_found"},
		{name: "menu item not found", err: ErrMenuItemNotFound, status: http.StatusNotFound, code: "menu_item_not_found"},
		{name: "order not found", err: ErrNotFound, status: http.StatusNotFound, code: "order_not_found"},
		{name: "restaurant unavailable", err: ErrRestaurantUnavailable, status: http.StatusConflict, code: "restaurant_unavailable"},
		{name: "menu item unavailable", err: ErrMenuItemUnavailable, status: http.StatusConflict, code: "menu_item_unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &orderServiceStub{err: test.err}
			router := testOrderRouter(service)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+testOrderID, nil))

			var body errorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Code != test.status || body.Code != test.code {
				t.Fatalf("unexpected response: %d %#v", response.Code, body)
			}
		})
	}
}

func TestGetOrderReturnsOrder(t *testing.T) {
	service := &orderServiceStub{order: testOrder(StatusConfirmed)}
	router := testOrderRouter(service)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+testOrderID, nil))

	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(testOrderID)) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestHandlerListOrdersReturnsCustomerOrders(t *testing.T) {
	service := &orderServiceStub{orders: []Order{testOrder(StatusConfirmed)}}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders?customer_id="+testCustomerID, nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if service.customerID != testCustomerID {
		t.Fatalf("unexpected customer id: %s", service.customerID)
	}
	var body ordersResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Orders) != 1 || body.Orders[0].ID != testOrderID {
		t.Fatalf("unexpected orders: %#v", body.Orders)
	}
}

func TestHandlerListOrdersReturnsEmptyArray(t *testing.T) {
	service := &orderServiceStub{orders: []Order{}}
	router := testOrderRouter(service)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/orders?customer_id="+testCustomerID, nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"orders":[]`)) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func testOrderRouter(service OrderService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(service).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func validCreateOrderJSON() string {
	return `{
		"customer_id":"50000000-0000-0000-0000-000000000001",
		"restaurant_id":"10000000-0000-0000-0000-000000000001",
		"items":[{"menu_item_id":"30000000-0000-0000-0000-000000000001","quantity":1}]
	}`
}
