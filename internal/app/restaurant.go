package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/examplerestaurant"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/config"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/httpserver"
)

func RunRestaurant(ctx context.Context, cfg config.Restaurant, logger *slog.Logger) error {
	router := NewRestaurantRouter(logger)
	logger.Info("starting example restaurant", "address", cfg.HTTPAddress)
	return httpserver.Run(ctx, cfg.HTTPAddress, router, cfg.ShutdownTimeout)
}

func NewRestaurantRouter(logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(httpserver.Recovery(logger), httpserver.RequestContext(logger))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "route_not_found", "message": "route was not found"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"code": "method_not_allowed", "message": "method is not allowed"})
	})
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "example-restaurant"})
	})

	handler := examplerestaurant.NewHandler()
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}
