package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type API struct {
	HTTPAddress              string
	DatabaseURL              string
	RestaurantRequestTimeout time.Duration
	SwaggerOrigin            string
	ShutdownTimeout          time.Duration
}

type Restaurant struct {
	HTTPAddress     string
	ShutdownTimeout time.Duration
}

func LoadAPI() (API, error) {
	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return API{}, err
	}
	restaurantTimeout, err := duration("RESTAURANT_REQUEST_TIMEOUT", 3*time.Second)
	if err != nil {
		return API{}, err
	}

	cfg := API{
		HTTPAddress:              env("HTTP_ADDRESS", ":8080"),
		DatabaseURL:              env("DATABASE_URL", "postgres://kitchen:kitchen@localhost:5432/kitchen?sslmode=disable"),
		RestaurantRequestTimeout: restaurantTimeout,
		SwaggerOrigin:            env("SWAGGER_ORIGIN", "http://localhost:8082"),
		ShutdownTimeout:          shutdownTimeout,
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		return API{}, fmt.Errorf("DATABASE_URL must not be empty")
	}
	return cfg, nil
}

func LoadRestaurant() (Restaurant, error) {
	timeout, err := duration("SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Restaurant{}, err
	}
	return Restaurant{
		HTTPAddress:     env("HTTP_ADDRESS", ":8081"),
		ShutdownTimeout: timeout,
	}, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	value := env(key, fallback.String())
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return parsed, nil
}
