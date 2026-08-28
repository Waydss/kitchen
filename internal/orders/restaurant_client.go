package orders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type HTTPRestaurantClient struct {
	client *http.Client
}

type restaurantOrderRequest struct {
	OrderID string                `json:"order_id"`
	Items   []restaurantOrderItem `json:"items"`
}

type restaurantOrderItem struct {
	ExternalItemID string `json:"external_item_id"`
	Quantity       int    `json:"quantity"`
}

type restaurantOrderResponse struct {
	ExternalOrderID string `json:"external_order_id"`
	Status          string `json:"status"`
}

type restaurantErrorResponse struct {
	Code string `json:"code"`
}

func NewHTTPRestaurantClient(client *http.Client) *HTTPRestaurantClient {
	return &HTTPRestaurantClient{client: client}
}

func (c *HTTPRestaurantClient) SubmitOrder(ctx context.Context, order Order) (RestaurantDecision, error) {
	payload := restaurantOrderRequest{OrderID: order.ID, Items: make([]restaurantOrderItem, 0, len(order.Items))}
	for _, item := range order.Items {
		payload.Items = append(payload.Items, restaurantOrderItem{
			ExternalItemID: item.ExternalItemID,
			Quantity:       item.Quantity,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return RestaurantDecision{}, fmt.Errorf("encode restaurant order: %w", err)
	}

	url := strings.TrimRight(order.RestaurantIntegrationURL, "/") + "/orders"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return RestaurantDecision{}, fmt.Errorf("create restaurant request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return RestaurantDecision{}, fmt.Errorf("submit restaurant order: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	limitedBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return RestaurantDecision{}, fmt.Errorf("read restaurant response: %w", err)
	}
	if response.StatusCode == http.StatusConflict || response.StatusCode == http.StatusBadRequest {
		var partnerError restaurantErrorResponse
		if err = json.Unmarshal(limitedBody, &partnerError); err != nil || partnerError.Code == "" {
			return RestaurantDecision{}, ErrInvalidPartnerResponse
		}
		switch partnerError.Code {
		case "item_unavailable", "invalid_order":
			return RestaurantDecision{Accepted: false, RejectionReason: partnerError.Code}, nil
		default:
			return RestaurantDecision{}, ErrInvalidPartnerResponse
		}
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return RestaurantDecision{}, fmt.Errorf("restaurant returned status %d: %w", response.StatusCode, ErrPartnerUnavailable)
	}

	var result restaurantOrderResponse
	if err = json.Unmarshal(limitedBody, &result); err != nil {
		return RestaurantDecision{}, fmt.Errorf("decode restaurant response: %w", ErrInvalidPartnerResponse)
	}
	if result.ExternalOrderID == "" || (result.Status != "accepted" && result.Status != "confirmed") {
		return RestaurantDecision{}, ErrInvalidPartnerResponse
	}
	return RestaurantDecision{Accepted: true, ExternalOrderID: result.ExternalOrderID}, nil
}

var _ RestaurantClient = (*HTTPRestaurantClient)(nil)
