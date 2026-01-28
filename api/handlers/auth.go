package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/auth"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userRepo    *queries.UserRepository
	authService *auth.Service
}

func NewAuthHandler(userRepo *queries.UserRepository, authService *auth.Service) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		authService: authService,
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	Username  string `json:"username"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if err == queries.ErrUserNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Set secure HTTP-only cookie with the token
	// Cookie expires in 24 hours (same as token)
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		"auth_token",   // name
		token,          // value
		86400,          // maxAge (24 hours in seconds)
		"/",            // path
		"",             // domain (empty = current domain)
		true,           // secure (HTTPS only)
		true,           // httpOnly (not accessible via JavaScript)
	)

	// Keep JSON response for backward compatibility
	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresIn: 86400, // 24 hours
		Username:  user.Username,
	})
}