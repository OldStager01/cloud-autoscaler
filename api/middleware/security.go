package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds essential security headers to protect against common vulnerabilities
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")
		
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection in browsers
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Enforce HTTPS (Strict-Transport-Security)
		// Only set this in production with HTTPS enabled
		// c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		
		// Content Security Policy - restrict resource loading
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " + // Allow inline scripts for Swagger
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: https:; " +
			"font-src 'self' data:; " +
			"connect-src 'self' ws: wss:; " + // Allow WebSocket connections
			"frame-ancestors 'none'"
		c.Header("Content-Security-Policy", csp)
		
		// Referrer Policy - control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy - control browser features
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		c.Next()
	}
}

// RequestSizeLimit middleware limits the size of request bodies to prevent DoS attacks
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(413, gin.H{
				"error": fmt.Sprintf("request body too large, maximum %d bytes allowed", maxBytes),
			})
			return
		}
		
		// Also set MaxBytesReader to enforce the limit during body reading
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		
		c.Next()
	}
}
