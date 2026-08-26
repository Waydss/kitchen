package examplerestaurant

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	mu     sync.Mutex
	orders map[string]orderResponse
	stock  map[string]int
}

type orderRequest struct {
	OrderID string      `json:"order_id" binding:"required,uuid"`
	Items   []orderItem `json:"items" binding:"required,min=1,dive"`
}

type orderItem struct {
	ExternalItemID string `json:"external_item_id" binding:"required"`
	Quantity       int    `json:"quantity" binding:"required,min=1,max=20"`
}

type orderResponse struct {
	ExternalOrderID string `json:"external_order_id"`
	Status          string `json:"status"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewHandler() *Handler {
	return &Handler{
		orders: make(map[string]orderResponse),
		stock: map[string]int{
			"margherita": 50,
			"pepperoni":  40,
			"lemonade":   100,
		},
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/info", h.info)
	group.POST("/orders", h.createOrder)
}

func (h *Handler) info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"external_restaurant_id": "demo-pizzeria",
		"name":                   "Demo Pizzeria",
	})
}

func (h *Handler) createOrder(c *gin.Context) {
	var request orderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_request", Message: "invalid order payload"})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.orders[request.OrderID]; ok {
		c.JSON(http.StatusOK, existing)
		return
	}
	for _, item := range request.Items {
		available, ok := h.stock[item.ExternalItemID]
		if !ok || available < item.Quantity {
			c.JSON(http.StatusConflict, errorResponse{Code: "item_unavailable", Message: "one or more items are unavailable"})
			return
		}
	}
	for _, item := range request.Items {
		h.stock[item.ExternalItemID] -= item.Quantity
	}

	response := orderResponse{ExternalOrderID: uuid.NewString(), Status: "accepted"}
	h.orders[request.OrderID] = response
	c.JSON(http.StatusCreated, response)
}
