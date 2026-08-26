package orders

import "time"

const (
	StatusPending   = "restaurant_confirmation_pending"
	StatusConfirmed = "confirmed"
	StatusRejected  = "rejected"
)

type Order struct {
	ID                       string          `json:"id"`
	CustomerID               string          `json:"customer_id"`
	RestaurantID             string          `json:"restaurant_id"`
	Status                   string          `json:"status"`
	TotalPrice               int64           `json:"total_price"`
	ExternalOrderID          string          `json:"external_order_id,omitempty"`
	Items                    []OrderItem     `json:"items"`
	StatusHistory            []StatusHistory `json:"status_history"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	RestaurantIntegrationURL string          `json:"-"`
}

type OrderItem struct {
	MenuItemID     string `json:"menu_item_id"`
	ExternalItemID string `json:"-"`
	Name           string `json:"name"`
	UnitPrice      int64  `json:"unit_price"`
	Quantity       int    `json:"quantity"`
}

type StatusHistory struct {
	PreviousStatus string    `json:"previous_status,omitempty"`
	Status         string    `json:"status"`
	Source         string    `json:"source"`
	CreatedAt      time.Time `json:"created_at"`
}

type CreateCommand struct {
	CustomerID     string
	RestaurantID   string
	IdempotencyKey string
	Items          []CreateItem
}

type CreateItem struct {
	MenuItemID string
	Quantity   int
}

type CreateResult struct {
	Order   Order
	Created bool
	Pending bool
}

type RestaurantDecision struct {
	Accepted        bool
	ExternalOrderID string
}
