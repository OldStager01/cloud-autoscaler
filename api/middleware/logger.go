package middleware

import (
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := map[string]interface{}{
			"status":     status,
			"method":     c.Request.Method,
			"path":       path,
			"latency_ms": latency.Milliseconds(),
			"ip":         c.ClientIP(),
		}

		if query != "" {
			fields["query"] = query
		}

		if traceID, exists := c.Get("trace_id"); exists {
			fields["trace_id"] = traceID
		}

		if len(c.Errors) > 0 {
			fields["errors"] = c.Errors.String()
		}

		entry := logger.WithFields(fields)

		switch {
		case status >= 500:
			entry.Error("server error")
		case status >= 400:
			entry.Warn("client error")
		default:
			entry.Info("request completed")
		}
	}
}