package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("panic recovered",
			"request_id", c.Writer.Header().Get("X-Request-ID"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"panic", fmt.Sprint(recovered),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
	})
}

func RequestContext(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)

		started := time.Now()
		c.Next()
		attributes := []any{
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
		}
		if c.Writer.Status() >= http.StatusInternalServerError && len(c.Errors) > 0 {
			attributes = append(attributes, "error", c.Errors.Last().Err)
			logger.Error("http request failed", attributes...)
			return
		}
		logger.Info("http request", attributes...)
	}
}
