package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"avito-kitchen/internal/catalog"
	"avito-kitchen/internal/orders"
	"avito-kitchen/internal/platform/config"
	"avito-kitchen/internal/platform/httpserver"
	"avito-kitchen/internal/platform/postgres"
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
	router := gin.New()
	router.Use(gin.Recovery(), httpserver.CORS(swaggerOrigin), httpserver.RequestContext(logger))

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
