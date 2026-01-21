package models

import (
	"time"

	"github.com/google/uuid"
)

// NewUUID generates a new UUID string
func NewUUID() string {
	return uuid.New().String()
}

// Timestamps contains common time fields
type Timestamps struct {
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}