package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestContextPreservesOrCreatesRequestID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, requestID := range []string{"", "client-request-id"} {
		t.Run(requestID, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RequestContext(logger))
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Request-ID", requestID)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			actual := response.Header().Get("X-Request-ID")
			if actual == "" || (requestID != "" && actual != requestID) {
				t.Fatalf("unexpected request id: %q", actual)
			}
		})
	}
}

func TestRecoveryHidesPanicDetails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery(logger))
	router.GET("/panic", func(*gin.Context) { panic("database password") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError || response.Body.String() != `{"code":"internal_error","message":"internal server error"}` {
		t.Fatalf("unexpected panic response: %d %s", response.Code, response.Body.String())
	}
}
