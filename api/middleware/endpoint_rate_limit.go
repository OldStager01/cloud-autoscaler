package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// EndpointRateLimiter provides per-endpoint rate limiting
type EndpointRateLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

func NewEndpointRateLimiter() *EndpointRateLimiter {
	return &EndpointRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

// AddEndpoint adds rate limiting configuration for a specific endpoint
func (erl *EndpointRateLimiter) AddEndpoint(path string, limit int, window time.Duration) {
	erl.mu.Lock()
	defer erl.mu.Unlock()
	erl.limiters[path] = NewRateLimiter(limit, window)
}

// Middleware returns a Gin middleware that enforces endpoint-specific rate limits
func (erl *EndpointRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		
		erl.mu.RLock()
		limiter, exists := erl.limiters[path]
		erl.mu.RUnlock()
		
		if exists {
			key := c.ClientIP()
			if !limiter.Allow(key) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error": "rate limit exceeded for this endpoint",
					"retry_after": limiter.window.Seconds(),
				})
				return
			}
		}
		
		c.Next()
	}
}

// AuthRateLimiter provides stricter rate limiting for authentication endpoints
func AuthRateLimiter() gin.HandlerFunc {
	// 5 requests per minute per IP for auth endpoints
	limiter := NewRateLimiter(5, time.Minute)
	
	return func(c *gin.Context) {
		key := c.ClientIP()
		
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many authentication attempts, please try again later",
				"retry_after": 60,
			})
			return
		}
		
		c.Next()
	}
}
