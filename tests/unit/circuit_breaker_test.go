package unit

import (
	"errors"
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
)

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     5 * time.Second,
	})

	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if cb.State() != resilience.StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     5 * time.Second,
	})

	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return testErr })
	}

	if cb.State() != resilience.StateOpen {
		t.Errorf("expected StateOpen, got %v", cb.State())
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

	if err != resilience.ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     50 * time.Millisecond,
	})

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	time.Sleep(100 * time.Millisecond)

	cb.Execute(func() error { return nil })

	if cb.State() != resilience.StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpen_Success(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     50 * time.Millisecond,
		HalfOpenMax: 2,
	})

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return nil })
	}

	if cb.State() != resilience.StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     1 * time.Hour,
	})

	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return errors.New("fail") })
	}

	cb.Reset()

	if cb.State() != resilience.StateClosed {
		t.Errorf("expected StateClosed after reset, got %v", cb.State())
	}
}
