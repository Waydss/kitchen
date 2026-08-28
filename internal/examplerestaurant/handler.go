package examplerestaurant

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	mu     sync.Mutex
	orders map[string]storedOrder
	stock  map[string]int
}

type storedOrder struct {
	accepted    *orderResponse
	rejected    *errorResponse
	fingerprint string
}

type orderRequest struct {
	OrderID string      `json:"order_id" binding:"required,uuid"`
	Items   []orderItem `json:"items" binding:"required,min=1,max=50,dive"`
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
		orders: make(map[string]storedOrder),
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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var request orderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_request", Message: "invalid order payload"})
		return
	}
	fingerprint, valid := orderFingerprint(request.Items)
	if !valid {
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_order", Message: "duplicate external_item_id"})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.orders[request.OrderID]; ok {
		if existing.fingerprint != fingerprint {
			c.JSON(http.StatusConflict, errorResponse{
				Code:    "idempotency_conflict",
				Message: "order_id was used for another payload",
			})
			return
		}
		if existing.rejected != nil {
			c.JSON(http.StatusConflict, existing.rejected)
			return
		}
		c.JSON(http.StatusOK, existing.accepted)
		return
	}
	for _, item := range request.Items {
		available, ok := h.stock[item.ExternalItemID]
		if !ok || available < item.Quantity {
			rejection := &errorResponse{Code: "item_unavailable", Message: "one or more items are unavailable"}
			h.orders[request.OrderID] = storedOrder{rejected: rejection, fingerprint: fingerprint}
			c.JSON(http.StatusConflict, rejection)
			return
		}
	}
	for _, item := range request.Items {
		h.stock[item.ExternalItemID] -= item.Quantity
	}

	response := &orderResponse{ExternalOrderID: uuid.NewString(), Status: "accepted"}
	h.orders[request.OrderID] = storedOrder{accepted: response, fingerprint: fingerprint}
	c.JSON(http.StatusCreated, response)
}

func orderFingerprint(items []orderItem) (string, bool) {
	normalized := append([]orderItem(nil), items...)
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].ExternalItemID < normalized[j].ExternalItemID
	})
	for i := 1; i < len(normalized); i++ {
		if normalized[i-1].ExternalItemID == normalized[i].ExternalItemID {
			return "", false
		}
	}
	for _, item := range normalized {
		if strings.TrimSpace(item.ExternalItemID) == "" {
			return "", false
		}
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), true
}
