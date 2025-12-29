package policies

import (
	"context"
	"testing"
	"time"

	"gosentry"
)

func TestRateLimit(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		ctx := context.Background()
		policy := RateLimit(RateLimitOptions{
			Rate:  10,
			Burst: 5,
		})

		handler := func(ctx context.Context) (any, error) {
			return "ok", nil
		}

		wrapped := policy(handler)

		for i := 0; i < 5; i++ {
			res, err := wrapped(ctx)
			if err != nil {
				t.Fatalf("expected no error on attempt %d, got %v", i, err)
			}
			if res != "ok" {
				t.Fatalf("expected ok, got %v", res)
			}
		}
	})

	t.Run("rejects requests exceeding burst", func(t *testing.T) {
		ctx := context.Background()
		policy := RateLimit(RateLimitOptions{
			Rate:  1,
			Burst: 2,
		})

		handler := func(ctx context.Context) (any, error) {
			return "ok", nil
		}

		wrapped := policy(handler)

		// Use up the burst
		wrapped(ctx)
		wrapped(ctx)

		// Third request should be rejected
		_, err := wrapped(ctx)
		if err != ErrRateLimitExceeded {
			t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
		}
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now()
		mockNow := func() time.Time {
			return now
		}

		policy := RateLimit(RateLimitOptions{
			Rate:  1,
			Burst: 1,
			Now:   mockNow,
		})

		handler := func(ctx context.Context) (any, error) {
			return "ok", nil
		}

		wrapped := policy(handler)

		// Use the only token
		wrapped(ctx)

		// Should be rejected now
		_, err := wrapped(ctx)
		if err != ErrRateLimitExceeded {
			t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
		}

		// Advance time by 1 second
		now = now.Add(time.Second)

		// Should be allowed now
		res, err := wrapped(ctx)
		if err != nil {
			t.Fatalf("expected no error after refill, got %v", err)
		}
		if res != "ok" {
			t.Fatalf("expected ok, got %v", res)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		policy := RateLimit(RateLimitOptions{
			Rate:  10,
			Burst: 10,
		})

		handler := func(ctx context.Context) (any, error) {
			return "ok", nil
		}

		wrapped := policy(handler)

		_, err := wrapped(ctx)
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestRateLimit_Integration(t *testing.T) {
	ctx := context.Background()
	
	rateLimit := RateLimit(RateLimitOptions{
		Rate:  100,
		Burst: 1,
	})

	handler := func(ctx context.Context) (any, error) {
		return "ok", nil
	}

	// First call should succeed
	res, err := gosentry.Execute(ctx, handler, rateLimit)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if res != "ok" {
		t.Fatalf("expected ok, got %v", res)
	}

	// Second call should fail immediately (burst is 1)
	_, err = gosentry.Execute(ctx, handler, rateLimit)
	if err != ErrRateLimitExceeded {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}
}

