package orders

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHTTPRestaurantClientSubmitsInternalIdentifiers(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v1/orders" {
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		return response(http.StatusCreated, `{"external_order_id":"restaurant-order-1","status":"accepted"}`), nil
	})}

	client := NewHTTPRestaurantClient(httpClient)
	decision, err := client.SubmitOrder(context.Background(), Order{
		ID:                       testOrderID,
		RestaurantIntegrationURL: "http://restaurant:8081/api/v1",
		Items:                    []OrderItem{{ExternalItemID: "margherita", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("submit order: %v", err)
	}
	if !decision.Accepted || decision.ExternalOrderID != "restaurant-order-1" {
		t.Fatalf("unexpected decision: %#v", decision)
	}
}

func TestHTTPRestaurantClientMapsUnavailableItemToRejection(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusConflict, `{"code":"item_unavailable","message":"unavailable"}`), nil
	})}

	decision, err := NewHTTPRestaurantClient(httpClient).SubmitOrder(context.Background(), Order{
		ID:                       testOrderID,
		RestaurantIntegrationURL: "http://restaurant:8081/api/v1",
	})
	if err != nil {
		t.Fatalf("submit rejected order: %v", err)
	}
	if decision.Accepted || decision.RejectionReason != "item_unavailable" {
		t.Fatalf("expected rejected decision: %#v", decision)
	}
}

func TestHTTPRestaurantClientRejectsUnknownPartnerError(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusConflict, `{"code":"unexpected_error"}`), nil
	})}

	_, err := NewHTTPRestaurantClient(httpClient).SubmitOrder(context.Background(), Order{
		ID:                       testOrderID,
		RestaurantIntegrationURL: "http://restaurant:8081/api/v1",
	})
	if !errors.Is(err, ErrInvalidPartnerResponse) {
		t.Fatalf("expected invalid response error, got %v", err)
	}
}

func TestHTTPRestaurantClientRejectsInvalidResponse(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusCreated, `{"status":"unexpected"}`), nil
	})}

	_, err := NewHTTPRestaurantClient(httpClient).SubmitOrder(context.Background(), Order{
		ID:                       testOrderID,
		RestaurantIntegrationURL: "http://restaurant:8081/api/v1",
	})
	if !errors.Is(err, ErrInvalidPartnerResponse) {
		t.Fatalf("expected invalid response error, got %v", err)
	}
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
