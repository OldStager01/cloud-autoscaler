package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const TraceIDHeader = "X-Trace-ID"

func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeader)
		if traceID == "" {
			traceID = uuid.New().String()
		}

		c.Set("trace_id", traceID)
		c.Header(TraceIDHeader, traceID)

		c.Next()
	}
}

func GetTraceID(c *gin.Context) string {
	if traceID, exists := c.Get("trace_id"); exists {
		return traceID.(string)
	}
	return ""
}