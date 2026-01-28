package middleware

import (
	"net/http"
	"strings"

	"github.com/OldStager01/cloud-autoscaler/internal/auth"
	"github.com/gin-gonic/gin"
)

const (
	AuthorizationHeader = "Authorization"
	BearerPrefix        = "Bearer "
	AuthCookieName      = "auth_token"
	UserIDKey           = "user_id"
	UsernameKey         = "username"
)

func JWTAuth(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// First, try to get token from Authorization header
		header := c.GetHeader(AuthorizationHeader)
		if header != "" {
			if !strings.HasPrefix(header, BearerPrefix) {
				// c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				// 	"error": "invalid authorization header format",
				// })
				// return
			}else{
				token = strings.TrimPrefix(header, BearerPrefix)
			}
		}

		// If no Authorization header, try to get token from cookie
		if token == ""  {
			cookieToken, err := c.Cookie(AuthCookieName)
			if err == nil && cookieToken != "" {
				token = cookieToken
			}
		}

		// If no token found in either location
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header or cookie",
			})
			return
		}
		claims, err := authService.ValidateToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			message := "invalid token"

			if err == auth.ErrExpiredToken {
				message = "token expired"
			}

			c.AbortWithStatusJSON(status, gin.H{
				"error": message,
			})
			return
		}

		c.Set(UserIDKey, claims.UserID)
		c.Set(UsernameKey, claims.Username)

		c.Next()
	}
}

func GetUserID(c *gin.Context) int {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return 0
	}
	return userID.(int)
}

func GetUsername(c *gin.Context) string {
	username, exists := c.Get(UsernameKey)
	if !exists {
		return ""
	}
	return username.(string)
}