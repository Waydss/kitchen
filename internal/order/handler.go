package order

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/httpserver"
)

type OrderService interface {
	Create(ctx context.Context, command CreateCommand) (CreateResult, error)
	ListByCustomerID(ctx context.Context, customerID string) ([]Order, error)
	Get(ctx context.Context, id string) (Order, error)
}

type Handler struct {
	service OrderService
}

type createOrderRequest struct {
	CustomerID   string               `json:"customer_id" binding:"required,uuid"`
	RestaurantID string               `json:"restaurant_id" binding:"required,uuid"`
	Items        []createOrderItemDTO `json:"items" binding:"required,min=1,max=50,dive"`
}

type createOrderItemDTO struct {
	MenuItemID string `json:"menu_item_id" binding:"required,uuid"`
	Quantity   int    `json:"quantity" binding:"required,min=1,max=20"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ordersResponse struct {
	Orders []Order `json:"orders"`
}

func NewHandler(service OrderService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/orders", h.createOrder)
	group.GET("/orders", h.listOrders)
	group.GET("/orders/:orderId", h.getOrder)
}

func (h *Handler) createOrder(c *gin.Context) {
	var request createOrderRequest
	if err := httpserver.BindJSON(c, &request, 1<<20); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, errorResponse{Code: "request_too_large", Message: "request body exceeds 1 MiB"})
			return
		}
		if errors.Is(err, httpserver.ErrUnsupportedMediaType) {
			c.JSON(http.StatusUnsupportedMediaType, errorResponse{Code: "unsupported_media_type", Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_request", Message: "invalid order payload"})
		return
	}

	items := make([]CreateItem, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, CreateItem(item))
	}
	result, err := h.service.Create(c.Request.Context(), CreateCommand{
		CustomerID:     request.CustomerID,
		RestaurantID:   request.RestaurantID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		Items:          items,
	})
	if errors.Is(err, ErrPartnerUnavailable) && result.Order.ID != "" {
		_ = c.Error(err)
		c.JSON(http.StatusAccepted, result.Order)
		return
	}
	if err != nil {
		writeError(c, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	c.JSON(status, result.Order)
}

func (h *Handler) getOrder(c *gin.Context) {
	order, err := h.service.Get(c.Request.Context(), c.Param("orderId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (h *Handler) listOrders(c *gin.Context) {
	orders, err := h.service.ListByCustomerID(c.Request.Context(), c.Query("customer_id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, ordersResponse{Orders: orders})
}

func writeError(c *gin.Context, err error) {
	_ = c.Error(err)
	switch {
	case errors.Is(err, ErrInvalidRequest):
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_request", Message: "invalid request"})
	case errors.Is(err, ErrRestaurantNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Code: "restaurant_not_found", Message: "restaurant was not found"})
	case errors.Is(err, ErrMenuItemNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Code: "menu_item_not_found", Message: "menu item was not found"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Code: "order_not_found", Message: "order was not found"})
	case errors.Is(err, ErrRestaurantUnavailable):
		c.JSON(http.StatusConflict, errorResponse{Code: "restaurant_unavailable", Message: "restaurant is temporarily unavailable"})
	case errors.Is(err, ErrMenuItemUnavailable):
		c.JSON(http.StatusConflict, errorResponse{Code: "menu_item_unavailable", Message: "menu item is unavailable"})
	case errors.Is(err, ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, errorResponse{Code: "idempotency_conflict", Message: "Idempotency-Key was used for another request"})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: "internal server error"})
	}
}
