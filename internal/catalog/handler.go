package catalog

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CatalogService interface {
	ListRestaurants(ctx context.Context) ([]Restaurant, error)
	GetRestaurant(ctx context.Context, id string) (Restaurant, error)
	GetMenu(ctx context.Context, restaurantID string) (Menu, error)
}

type Handler struct {
	service CatalogService
}

func NewHandler(service CatalogService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/restaurants", h.listRestaurants)
	group.GET("/restaurants/:restaurantId", h.getRestaurant)
	group.GET("/restaurants/:restaurantId/menu", h.getMenu)
}

func (h *Handler) listRestaurants(c *gin.Context) {
	restaurants, err := h.service.ListRestaurants(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurants": restaurants})
}

func (h *Handler) getRestaurant(c *gin.Context) {
	restaurant, err := h.service.GetRestaurant(c.Request.Context(), c.Param("restaurantId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, restaurant)
}

func (h *Handler) getMenu(c *gin.Context) {
	menu, err := h.service.GetMenu(c.Request.Context(), c.Param("restaurantId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, menu)
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidID):
		c.JSON(http.StatusBadRequest, errorResponse{Code: "invalid_argument", Message: "restaurantId must be a UUID"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Code: "restaurant_not_found", Message: "restaurant was not found"})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Code: "internal_error", Message: "internal server error"})
	}
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
