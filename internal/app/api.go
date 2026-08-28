package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/catalog"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/orders"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/config"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/httpserver"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/platform/postgres"
)

func RunAPI(ctx context.Context, cfg config.API, logger *slog.Logger) error {
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	restaurantClient := orders.NewHTTPRestaurantClient(&http.Client{Timeout: cfg.RestaurantRequestTimeout})
	router := NewAPIRouter(pool, restaurantClient, cfg.SwaggerOrigin, logger)
	logger.Info("starting api", "address", cfg.HTTPAddress)
	return httpserver.Run(ctx, cfg.HTTPAddress, router, cfg.ShutdownTimeout)
}

func NewAPIRouter(
	pool *pgxpool.Pool,
	restaurantClient orders.RestaurantClient,
	swaggerOrigin string,
	logger *slog.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	gin.EnableJsonDecoderDisallowUnknownFields()
	router := gin.New()
	router.Use(httpserver.Recovery(logger), httpserver.CORS(swaggerOrigin), httpserver.RequestContext(logger))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "route_not_found", "message": "route was not found"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"code": "method_not_allowed", "message": "method is not allowed"})
	})

	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "avito-kitchen-api"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	apiV1 := router.Group("/api/v1")

	catalogRepository := catalog.NewPostgresRepository(pool)
	catalogService := catalog.NewService(catalogRepository)
	catalogHandler := catalog.NewHandler(catalogService)
	catalogHandler.RegisterRoutes(apiV1)

	orderRepository := orders.NewPostgresRepository(pool)
	orderService := orders.NewService(orderRepository, restaurantClient)
	orderHandler := orders.NewHandler(orderService)
	orderHandler.RegisterRoutes(apiV1)

	return router
}
