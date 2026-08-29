package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type jsonFixture struct {
	Name string `json:"name" binding:"required"`
}

func TestBindJSONAcceptsOneValidObject(t *testing.T) {
	c := jsonTestContext(t, `{"name":"demo"}`, "application/json; charset=utf-8")
	var target jsonFixture

	if err := BindJSON(c, &target, 1024); err != nil {
		t.Fatalf("bind JSON: %v", err)
	}
	if target.Name != "demo" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestBindJSONRejectsUnsupportedMediaType(t *testing.T) {
	c := jsonTestContext(t, `{"name":"demo"}`, "text/plain")
	var target jsonFixture

	if err := BindJSON(c, &target, 1024); !errors.Is(err, ErrUnsupportedMediaType) {
		t.Fatalf("expected ErrUnsupportedMediaType, got %v", err)
	}
}

func TestBindJSONRejectsInvalidBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
		max  int64
	}{
		{name: "unknown field", body: `{"name":"demo","extra":true}`, max: 1024},
		{name: "trailing object", body: `{"name":"demo"}{"name":"second"}`, max: 1024},
		{name: "missing required field", body: `{}`, max: 1024},
		{name: "malformed", body: `{"name":`, max: 1024},
		{name: "oversized", body: `{"name":"demo"}`, max: 4},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := jsonTestContext(t, test.body, "application/json")
			var target jsonFixture
			if err := BindJSON(c, &target, test.max); err == nil {
				t.Fatal("expected binding error")
			}
		})
	}
}

func jsonTestContext(t *testing.T, body, contentType string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	return c
}
