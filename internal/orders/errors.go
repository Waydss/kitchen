package orders

import "errors"

var (
	ErrInvalidRequest         = errors.New("invalid order request")
	ErrNotFound               = errors.New("order not found")
	ErrRestaurantNotFound     = errors.New("restaurant not found")
	ErrRestaurantUnavailable  = errors.New("restaurant unavailable")
	ErrMenuItemNotFound       = errors.New("menu item not found")
	ErrMenuItemUnavailable    = errors.New("menu item unavailable")
	ErrIdempotencyConflict    = errors.New("idempotency key is already used for another request")
	ErrInvalidTransition      = errors.New("invalid order status transition")
	ErrPartnerUnavailable     = errors.New("restaurant service unavailable")
	ErrInvalidPartnerResponse = errors.New("invalid restaurant response")
)
