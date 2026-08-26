package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"avito-kitchen/internal/examplerestaurant"
	"avito-kitchen/internal/platform/config"
	"avito-kitchen/internal/platform/httpserver"
)

func RunRestaurant(ctx context.Context, cfg config.Restaurant, logger *slog.Logger) error {
	router := NewRestaurantRouter(logger)
	logger.Info("starting example restaurant", "address", cfg.HTTPAddress)
	return httpserver.Run(ctx, cfg.HTTPAddress, router, cfg.ShutdownTimeout)
}

func NewRestaurantRouter(logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), httpserver.RequestContext(logger))
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "example-restaurant"})
	})

	handler := examplerestaurant.NewHandler()
	handler.RegisterRoutes(router.Group("/api/v1"))
	return router
}
