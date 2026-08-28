package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type orderServiceStub struct {
	result  CreateResult
	order   Order
	err     error
	command CreateCommand
}

func (s *orderServiceStub) Create(_ context.Context, command CreateCommand) (CreateResult, error) {
	s.command = command
	return s.result, s.err
}

func (s *orderServiceStub) Get(context.Context, string) (Order, error) {
	return s.order, s.err
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
