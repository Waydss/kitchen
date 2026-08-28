package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAPIReadsEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDRESS", ":9090")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("RESTAURANT_REQUEST_TIMEOUT", "1500ms")
	t.Setenv("SWAGGER_ORIGIN", "https://docs.example.test")
	t.Setenv("SHUTDOWN_TIMEOUT", "7s")

	cfg, err := LoadAPI()
	if err != nil {
		t.Fatalf("LoadAPI: %v", err)
	}
	if cfg.HTTPAddress != ":9090" || cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.RestaurantRequestTimeout != 1500*time.Millisecond || cfg.ShutdownTimeout != 7*time.Second {
		t.Fatalf("unexpected durations: %#v", cfg)
	}
}

func TestLoadAPIRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "duration", key: "SHUTDOWN_TIMEOUT", value: "later", want: "parse SHUTDOWN_TIMEOUT"},
		{name: "non-positive duration", key: "RESTAURANT_REQUEST_TIMEOUT", value: "0s", want: "must be positive"},
		{name: "empty database URL", key: "DATABASE_URL", value: " ", want: "DATABASE_URL must not be empty"},
		{name: "invalid Swagger origin", key: "SWAGGER_ORIGIN", value: "localhost:8082/path", want: "must be an HTTP origin"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HTTP_ADDRESS", ":8080")
			t.Setenv("DATABASE_URL", "postgres://example")
			t.Setenv("RESTAURANT_REQUEST_TIMEOUT", "3s")
			t.Setenv("SWAGGER_ORIGIN", "http://localhost:8082")
			t.Setenv("SHUTDOWN_TIMEOUT", "10s")
			t.Setenv(test.key, test.value)

			_, err := LoadAPI()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLoadRestaurantRejectsEmptyAddress(t *testing.T) {
	t.Setenv("HTTP_ADDRESS", " ")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	_, err := LoadRestaurant()
	if err == nil || !strings.Contains(err.Error(), "HTTP_ADDRESS") {
		t.Fatalf("expected HTTP_ADDRESS error, got %v", err)
	}
}
