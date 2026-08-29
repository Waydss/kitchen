package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Kitchen struct {
	HTTP       KitchenHTTP
	Database   Database
	Restaurant RestaurantClient
	Swagger    Swagger
}

type KitchenHTTP struct {
	Address         string        `env:"HTTP_ADDRESS" env-default:":8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"10s"`
}

type Database struct {
	URL string `env:"DATABASE_URL" env-default:"postgres://kitchen:kitchen@localhost:5432/kitchen?sslmode=disable"`
}

type RestaurantClient struct {
	RequestTimeout time.Duration `env:"RESTAURANT_REQUEST_TIMEOUT" env-default:"3s"`
}

type Swagger struct {
	Origin string `env:"SWAGGER_ORIGIN" env-default:"http://localhost:8082"`
}

type Restaurant struct {
	HTTP RestaurantHTTP
}

type RestaurantHTTP struct {
	Address         string        `env:"HTTP_ADDRESS" env-default:":8081"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" env-default:"10s"`
}

func LoadKitchen() (Kitchen, error) {
	var cfg Kitchen
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Kitchen{}, fmt.Errorf("read kitchen config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Kitchen{}, err
	}
	return cfg, nil
}

func LoadRestaurant() (Restaurant, error) {
	var cfg Restaurant
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Restaurant{}, fmt.Errorf("read restaurant config: %w", err)
	}
	if strings.TrimSpace(cfg.HTTP.Address) == "" {
		return Restaurant{}, fmt.Errorf("HTTP_ADDRESS must not be empty")
	}
	if cfg.HTTP.ShutdownTimeout <= 0 {
		return Restaurant{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	return cfg, nil
}

func (cfg Kitchen) validate() error {
	if strings.TrimSpace(cfg.HTTP.Address) == "" {
		return fmt.Errorf("HTTP_ADDRESS must not be empty")
	}
	if strings.TrimSpace(cfg.Database.URL) == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}
	if cfg.HTTP.ShutdownTimeout <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	if cfg.Restaurant.RequestTimeout <= 0 {
		return fmt.Errorf("RESTAURANT_REQUEST_TIMEOUT must be positive")
	}
	if err := validateHTTPOrigin("SWAGGER_ORIGIN", cfg.Swagger.Origin); err != nil {
		return err
	}
	return nil
}

func validateHTTPOrigin(key, value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" {
		return fmt.Errorf("%s must be an HTTP origin, for example http://localhost:8082", key)
	}
	return nil
}
