package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/auth"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userRepo    *queries.UserRepository
	authService *auth.Service
	config      *config.APIConfig
}

func NewAuthHandler(userRepo *queries.UserRepository, authService *auth.Service, cfg *config.APIConfig) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		authService: authService,
		config:      cfg,
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"john_doe"`
	Password string `json:"password" binding:"required" example:"secretpassword123"`
}

type LoginResponse struct {
	Token     string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresIn int    `json:"expires_in" example:"86400"`
	Username  string `json:"username" example:"john_doe"`
}

// Login godoc
// @Summary User login
// @Description Authenticate user and return JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "Login successful"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 401 {object} map[string]string "Invalid credentials"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /auth/login [post]
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

	// Get cookie settings from config with defaults
	cookieName := h.config.CookieName
	if cookieName == "" {
		cookieName = "auth_token"
	}
	cookieMaxAge := h.config.CookieMaxAge
	if cookieMaxAge == 0 {
		cookieMaxAge = 86400 // 24 hours
	}
	cookiePath := h.config.CookiePath
	if cookiePath == "" {
		cookiePath = "/"
	}

	// Set secure HTTP-only cookie with the token
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		cookieName,              // name
		token,                   // value
		cookieMaxAge,            // maxAge
		cookiePath,              // path
		"",                      // domain (empty = current domain)
		h.config.CookieSecure,   // secure (HTTPS only)
		h.config.CookieHTTPOnly, // httpOnly (not accessible via JavaScript)
	)

	// Keep JSON response for backward compatibility
	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresIn: cookieMaxAge,
		Username:  user.Username,
	})
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"john_doe"`
	Password string `json:"password" binding:"required,min=6" example:"secretpassword123"`
}

type RegisterResponse struct {
	ID       int    `json:"id" example:"1"`
	Username string `json:"username" example:"john_doe"`
	Message  string `json:"message" example:"user registered successfully"`
}

// Register godoc
// @Summary Register new user
// @Description Create a new user account
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} RegisterResponse "User registered successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 409 {object} map[string]string "Username already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Check if username already exists
	existing, err := h.userRepo.GetByUsername(ctx, req.Username)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}
	if err != nil && err != queries.ErrUserNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Hash the password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process password"})
		return
	}

	// Create the user
	user, err := h.userRepo.Create(ctx, req.Username, passwordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
		Message:  "user registered successfully",
	})
}