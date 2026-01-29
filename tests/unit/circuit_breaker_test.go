package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
)

func TestCircuitBreaker_Execute(t *testing.T) {
	tests := []struct {
		name          string
		config        resilience.CircuitBreakerConfig
		execFunc      func() error
		expectedErr   error
		expectedState resilience.State
	}{
		{
			name: "successful execution stays closed",
			config: resilience.CircuitBreakerConfig{
				MaxFailures: 3,
				Timeout:     5 * time.Second,
			},
			execFunc:      func() error { return nil },
			expectedErr:   nil,
			expectedState: resilience.StateClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := resilience.NewCircuitBreaker(tt.config)

			err := cb.Execute(tt.execFunc)

			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedState, cb.State())
		})
	}
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		config        resilience.CircuitBreakerConfig
		setup         func(cb *resilience.CircuitBreaker)
		expectedState resilience.State
	}{
		{
			name: "transition to open after max failures",
			config: resilience.CircuitBreakerConfig{
				MaxFailures: 3,
				Timeout:     5 * time.Second,
			},
			setup: func(cb *resilience.CircuitBreaker) {
				for i := 0; i < 3; i++ {
					cb.Execute(func() error { return errors.New("fail") })
				}
			},
			expectedState: resilience.StateOpen,
		},
		{
			name: "transition to half-open after timeout",
			config: resilience.CircuitBreakerConfig{
				MaxFailures: 3,
				Timeout:     50 * time.Millisecond,
			},
			setup: func(cb *resilience.CircuitBreaker) {
				for i := 0; i < 3; i++ {
					cb.Execute(func() error { return errors.New("fail") })
				}
				time.Sleep(100 * time.Millisecond)
				cb.Execute(func() error { return nil })
			},
			expectedState: resilience.StateHalfOpen,
		},
		{
			name: "transition from half-open to closed on success",
			config: resilience.CircuitBreakerConfig{
				MaxFailures: 3,
				Timeout:     50 * time.Millisecond,
				HalfOpenMax: 2,
			},
			setup: func(cb *resilience.CircuitBreaker) {
				for i := 0; i < 3; i++ {
					cb.Execute(func() error { return errors.New("fail") })
				}
				time.Sleep(100 * time.Millisecond)
				for i := 0; i < 3; i++ {
					cb.Execute(func() error { return nil })
				}
			},
			expectedState: resilience.StateClosed,
		},
		{
			name: "reset returns to closed",
			config: resilience.CircuitBreakerConfig{
				MaxFailures: 3,
				Timeout:     1 * time.Hour,
			},
			setup: func(cb *resilience.CircuitBreaker) {
				for i := 0; i < 3; i++ {
					cb.Execute(func() error { return errors.New("fail") })
				}
				cb.Reset()
			},
			expectedState: resilience.StateClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := resilience.NewCircuitBreaker(tt.config)

			tt.setup(cb)

			assert.Equal(t, tt.expectedState, cb.State())
		})
	}
}

func TestCircuitBreaker_OpenState_RejectsRequest(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     1 * time.Hour,
	})

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	err := cb.Execute(func() error { return nil })

	assert.ErrorIs(t, err, resilience.ErrCircuitOpen)
}
