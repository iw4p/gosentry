package policies

import (
	"context"
	"errors"
	"testing"
	"time"

	"gosentry"
)

func TestRetry_FirstAttemptSuccessful(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	wrapped := policy(handler)
	result, err := wrapped(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_FirstFailsSecondSucceeds(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("first attempt failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	wrapped := policy(handler)
	start := time.Now()
	result, err := wrapped(context.Background())
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if duration < 10*time.Millisecond {
		t.Fatalf("expected delay between attempts, got %v", duration)
	}
}

func TestRetry_AllAttemptsFail(t *testing.T) {
	attempts := 0
	expectedErr := errors.New("all attempts failed")
	handler := func(ctx context.Context) (any, error) {
		attempts++
		return nil, expectedErr
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	wrapped := policy(handler)
	result, err := wrapped(context.Background())

	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_ContextCancellationDuringWait(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		return nil, errors.New("failed")
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	wrapped := policy(handler)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := wrapped(ctx)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
	if duration < 50*time.Millisecond || duration > 150*time.Millisecond {
		t.Fatalf("expected duration around 50ms, got %v", duration)
	}
}

func TestRetry_ContextCancellationBeforeRetry(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("first attempt failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 200 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	wrapped := policy(handler)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := wrapped(ctx)

	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt before cancellation, got %d", attempts)
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	attempts := 0
	delays := []time.Duration{}
	lastTime := time.Now()

	handler := func(ctx context.Context) (any, error) {
		if attempts > 0 {
			delays = append(delays, time.Since(lastTime))
		}
		attempts++
		lastTime = time.Now()
		if attempts < 3 {
			return nil, errors.New("failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffExponential,
		Jitter:       false,
	})

	wrapped := policy(handler)
	wrapped(context.Background())

	if len(delays) != 2 {
		t.Fatalf("expected 2 delays, got %d", len(delays))
	}

	expected1 := 10 * time.Millisecond
	expected2 := 20 * time.Millisecond

	if delays[0] < expected1 || delays[0] > expected1+5*time.Millisecond {
		t.Fatalf("expected first delay around %v, got %v", expected1, delays[0])
	}
	if delays[1] < expected2 || delays[1] > expected2+5*time.Millisecond {
		t.Fatalf("expected second delay around %v, got %v", expected2, delays[1])
	}
}

func TestRetry_LinearBackoff(t *testing.T) {
	attempts := 0
	delays := []time.Duration{}
	lastTime := time.Now()

	handler := func(ctx context.Context) (any, error) {
		if attempts > 0 {
			delays = append(delays, time.Since(lastTime))
		}
		attempts++
		lastTime = time.Now()
		if attempts < 3 {
			return nil, errors.New("failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffLinear,
		Jitter:       false,
	})

	wrapped := policy(handler)
	wrapped(context.Background())

	if len(delays) != 2 {
		t.Fatalf("expected 2 delays, got %d", len(delays))
	}

	expected1 := 10 * time.Millisecond
	expected2 := 20 * time.Millisecond

	if delays[0] < expected1 || delays[0] > expected1+5*time.Millisecond {
		t.Fatalf("expected first delay around %v, got %v", expected1, delays[0])
	}
	if delays[1] < expected2 || delays[1] > expected2+5*time.Millisecond {
		t.Fatalf("expected second delay around %v, got %v", expected2, delays[1])
	}
}

func TestRetry_MaxDelayClamping(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		return nil, errors.New("failed")
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     150 * time.Millisecond,
		Backoff:      BackoffExponential,
		Jitter:       false,
	})

	wrapped := policy(handler)
	start := time.Now()
	wrapped(context.Background())
	duration := time.Since(start)

	maxExpectedDuration := 4 * 150 * time.Millisecond
	if duration > maxExpectedDuration+100*time.Millisecond {
		t.Fatalf("expected duration capped by MaxDelay, got %v", duration)
	}
}

func TestRetry_DefaultOptions(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{})
	wrapped := policy(handler)
	result, err := wrapped(context.Background())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetry_WithResilienceExecute(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context) (any, error) {
		attempts++
		if attempts < 2 {
			return nil, errors.New("failed")
		}
		return "success", nil
	}

	policy := Retry(RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Backoff:      BackoffFixed,
		Jitter:       false,
	})

	result, err := gosentry.Execute(context.Background(), handler, policy)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Fatalf("expected 'success', got %v", result)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}
