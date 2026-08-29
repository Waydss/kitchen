package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/catalog"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/config"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/httpserver"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/order"
	"github.com/talense-tasks/backend-trainee-assignment-autumn-2026-waydss-e38eb47e/internal/postgres"
)

func RunKitchen(ctx context.Context, cfg config.Kitchen, logger *slog.Logger) error {
	pool, err := postgres.Open(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer pool.Close()

	restaurantClient := order.NewHTTPRestaurantClient(&http.Client{Timeout: cfg.Restaurant.RequestTimeout})
	router := NewKitchenRouter(pool, restaurantClient, cfg.Swagger.Origin, logger)
	logger.Info("starting kitchen", "address", cfg.HTTP.Address)
	return httpserver.Run(ctx, cfg.HTTP.Address, router, cfg.HTTP.ShutdownTimeout)
}

func NewKitchenRouter(
	pool *pgxpool.Pool,
	restaurantClient order.RestaurantClient,
	swaggerOrigin string,
	logger *slog.Logger,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.HandleMethodNotAllowed = true
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

	orderRepository := order.NewPostgresRepository(pool)
	orderService := order.NewService(orderRepository, restaurantClient)
	orderHandler := order.NewHandler(orderService)
	orderHandler.RegisterRoutes(apiV1)

	return router
}
