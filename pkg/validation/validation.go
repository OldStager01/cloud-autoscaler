package validation

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	// ErrInvalidInput indicates the input failed validation
	ErrInvalidInput = errors.New("invalid input")
	
	// Cluster name must be alphanumeric with hyphens/underscores, 3-100 chars
	clusterNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,99}$`)
	
	// Username must be alphanumeric with underscores, 3-50 chars
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,50}$`)
)

// SanitizeString removes potentially dangerous characters and trims whitespace
func SanitizeString(input string) string {
	// Trim whitespace
	input = strings.TrimSpace(input)
	
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Remove control characters except newline and tab
	var builder strings.Builder
	for _, r := range input {
		if !unicode.IsControl(r) || r == '\n' || r == '\t' {
			builder.WriteRune(r)
		}
	}
	
	return builder.String()
}

// ValidateClusterName checks if a cluster name is valid
func ValidateClusterName(name string) error {
	name = SanitizeString(name)
	
	if name == "" {
		return errors.New("cluster name cannot be empty")
	}
	
	if len(name) < 3 {
		return errors.New("cluster name must be at least 3 characters")
	}
	
	if len(name) > 100 {
		return errors.New("cluster name must not exceed 100 characters")
	}
	
	if !clusterNameRegex.MatchString(name) {
		return errors.New("cluster name must start with alphanumeric and contain only letters, numbers, hyphens, and underscores")
	}
	
	// Prevent reserved names
	reserved := []string{"admin", "root", "system", "default", "test"}
	lowerName := strings.ToLower(name)
	for _, r := range reserved {
		if lowerName == r {
			return errors.New("cluster name is reserved")
		}
	}
	
	return nil
}

// ValidateUsername checks if a username is valid
func ValidateUsername(username string) error {
	username = SanitizeString(username)
	
	if username == "" {
		return errors.New("username cannot be empty")
	}
	
	if len(username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	
	if len(username) > 50 {
		return errors.New("username must not exceed 50 characters")
	}
	
	// TODO: Temporarily disabled to allow email in username field
	// if !usernameRegex.MatchString(username) {
	// 	return errors.New("username must contain only letters, numbers, and underscores")
	// }
	
	return nil
}

// ValidatePassword checks if a password meets security requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	
	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}
	
	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)
	
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	
	return nil
}

// ValidateServerCount checks if min/max server counts are valid
func ValidateServerCount(min, max int) error {
	if min < 1 {
		return errors.New("minimum servers must be at least 1")
	}
	
	if max < min {
		return errors.New("maximum servers must be greater than or equal to minimum servers")
	}
	
	if max > 1000 {
		return errors.New("maximum servers cannot exceed 1000")
	}
	
	return nil
}
