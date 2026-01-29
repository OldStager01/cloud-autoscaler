package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OldStager01/cloud-autoscaler/internal/auth"
)

func TestService_GenerateToken(t *testing.T) {
	svc := auth.NewService("test-secret", time.Hour)

	token, err := svc.GenerateToken(1, "testuser")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestService_ValidateToken(t *testing.T) {
	tests := []struct {
		name          string
		tokenDuration time.Duration
		token         string
		useGenerated  bool
		expectedErr   error
		expectedUID   int
		expectedUser  string
	}{
		{
			name:          "valid token",
			tokenDuration: time.Hour,
			useGenerated:  true,
			expectedErr:   nil,
			expectedUID:   1,
			expectedUser:  "testuser",
		},
		{
			name:          "invalid token",
			tokenDuration: time.Hour,
			token:         "invalid-token",
			useGenerated:  false,
			expectedErr:   auth.ErrInvalidToken,
		},
		{
			name:          "expired token",
			tokenDuration: -time.Hour,
			useGenerated:  true,
			expectedErr:   auth.ErrExpiredToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService("test-secret", tt.tokenDuration)

			var token string
			if tt.useGenerated {
				var err error
				token, err = svc.GenerateToken(1, "testuser")
				require.NoError(t, err)
			} else {
				token = tt.token
			}

			claims, err := svc.ValidateToken(token)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUID, claims.UserID)
				assert.Equal(t, tt.expectedUser, claims.Username)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		check    string
		expected bool
	}{
		{
			name:     "correct password matches",
			password: "mypassword123",
			check:    "mypassword123",
			expected: true,
		},
		{
			name:     "wrong password does not match",
			password: "mypassword123",
			check:    "wrongpassword",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := auth.HashPassword(tt.password)
			require.NoError(t, err)

			result := auth.CheckPassword(tt.check, hash)
			assert.Equal(t, tt.expected, result)
		})
	}
}
