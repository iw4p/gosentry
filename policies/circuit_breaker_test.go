package policies

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_OpensAfterFailureThresholdAndRejects(t *testing.T) {
	callCount := 0
	handler := func(ctx context.Context) (any, error) {
		callCount++
		return nil, errors.New("boom")
	}

	policy := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 2,
		SuccessThreshold: 1,
		OpenTimeout:      10 * time.Second,
	})

	wrapped := policy(handler)

	// 1st failure: still closed
	_, err := wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}

	// 2nd failure: trips open (but this call executes)
	_, err = wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 2 {
		t.Fatalf("expected handler called twice, got %d", callCount)
	}

	// Now open: should reject without calling handler
	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected handler not called while open, got %d", callCount)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout_AllowsTrialThenClosesOnSuccess(t *testing.T) {
	now := time.Now()
	mu := sync.Mutex{}

	calls := 0
	policy := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		OpenTimeout:      50 * time.Millisecond,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	})
	wrapped := policy(func(ctx context.Context) (any, error) {
		// First call fails to open circuit; subsequent calls succeed.
		calls++
		if calls == 1 {
			return nil, errors.New("fail once")
		}
		return "ok", nil
	})

	// Trip open (threshold=1).
	_, err := wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	// Still open: reject.
	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Advance time past open timeout -> half-open and allow trial.
	mu.Lock()
	now = now.Add(60 * time.Millisecond)
	mu.Unlock()

	res, err := wrapped(context.Background())
	if err != nil {
		t.Fatalf("expected success in half-open trial, got %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected 'ok', got %v", res)
	}

	// Should be closed now; next call should go through (and succeed).
	res, err = wrapped(context.Background())
	if err != nil {
		t.Fatalf("expected success after closing, got %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected 'ok', got %v", res)
	}
}

func TestCircuitBreaker_HalfOpenBusyRejectsConcurrentCalls(t *testing.T) {
	now := time.Now()
	mu := sync.Mutex{}

	started := make(chan struct{})
	unblock := make(chan struct{})

	callCount := 0
	handler := func(ctx context.Context) (any, error) {
		callCount++
		close(started)
		<-unblock
		return "ok", nil
	}

	policy := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		OpenTimeout:      10 * time.Millisecond,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	})

	// First call fails and opens circuit.
	wrapped := policy(func(ctx context.Context) (any, error) {
		if callCount == 0 {
			callCount++
			return nil, errors.New("fail")
		}
		return handler(ctx)
	})

	_, err := wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error to open circuit")
	}

	// Advance time past open timeout to enter half-open.
	mu.Lock()
	now = now.Add(20 * time.Millisecond)
	mu.Unlock()

	// Start trial call and block it.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = wrapped(context.Background())
	}()

	<-started

	// Second call while half-open trial in-flight should be rejected.
	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitHalfOpenBusy) {
		t.Fatalf("expected ErrCircuitHalfOpenBusy, got %v", err)
	}

	close(unblock)
	wg.Wait()
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	now := time.Now()
	mu := sync.Mutex{}

	calls := 0
	wrapped := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		OpenTimeout:      10 * time.Millisecond,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	})(func(ctx context.Context) (any, error) {
		calls++
		// 1: fail -> open
		// 2: (open) rejected (handler not called)
		// 2: (half-open) fail -> re-open (this is the second handler invocation)
		if calls == 1 {
			return nil, errors.New("fail")
		}
		return nil, errors.New("still failing")
	})

	_, err := wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	mu.Lock()
	now = now.Add(20 * time.Millisecond)
	mu.Unlock()

	_, err = wrapped(context.Background())
	if err == nil {
		t.Fatal("expected error on half-open trial")
	}

	// After half-open failure, it should be open again.
	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen after re-open, got %v", err)
	}
}

func TestCircuitBreaker_ShouldTripFiltersFailures(t *testing.T) {
	tripErr := errors.New("trip")
	ignoredErr := errors.New("ignore")

	callCount := 0
	handler := func(ctx context.Context) (any, error) {
		callCount++
		if callCount == 1 {
			return nil, ignoredErr
		}
		return nil, tripErr
	}

	policy := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      10 * time.Second,
		ShouldTrip: func(err error) bool {
			return errors.Is(err, tripErr)
		},
	})
	wrapped := policy(handler)

	// ignoredErr should not trip
	_, err := wrapped(context.Background())
	if !errors.Is(err, ignoredErr) {
		t.Fatalf("expected ignoredErr, got %v", err)
	}

	// tripErr should open circuit (threshold=1)
	_, err = wrapped(context.Background())
	if !errors.Is(err, tripErr) {
		t.Fatalf("expected tripErr, got %v", err)
	}

	// now open -> reject
	_, err = wrapped(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ContextErrorTakesPrecedence(t *testing.T) {
	callCount := 0
	handler := func(ctx context.Context) (any, error) {
		callCount++
		return "ok", nil
	}

	wrapped := CircuitBreaker(CircuitBreakerOptions{
		FailureThreshold: 1,
		OpenTimeout:      10 * time.Second,
	})(handler)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wrapped(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if callCount != 0 {
		t.Fatalf("expected handler not called, got %d", callCount)
	}
}
