package unit

import (
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/auth"
)

func TestService_GenerateToken(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	token, err := svc.GenerateToken(1, "testuser")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestService_ValidateToken_Valid(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	token, _ := svc.GenerateToken(1, "testuser")
	claims, err := svc.ValidateToken(token)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", claims.Username)
	}
}

func TestService_ValidateToken_Invalid(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	_, err := svc.ValidateToken("invalid-token")

	if err != auth.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestService_ValidateToken_Expired(t *testing.T) {
	svc := auth.NewService("test-secret", -time.Hour)

	token, _ := svc.GenerateToken(1, "testuser")
	_, err := svc.ValidateToken(token)

	if err != auth.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestCheckPassword(t *testing.T) {
	password := "mypassword123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !auth.CheckPassword(password, hash) {
		t.Error("expected password to match")
	}
	if auth.CheckPassword("wrongpassword", hash) {
		t.Error("expected wrong password to not match")
	}
}
