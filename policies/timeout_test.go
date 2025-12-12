package policies

import (
	"context"
	"errors"
	"testing"
	"time"

	"gosentry"
)

func TestTimeout_CompletesBeforeDeadline(t *testing.T) {
	p := Timeout(TimeoutOptions{Duration: 50 * time.Millisecond})
	h := p(func(ctx context.Context) (any, error) {
		time.Sleep(5 * time.Millisecond)
		return "ok", nil
	})

	got, err := h(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %v", got)
	}
}

func TestTimeout_ExceedsDeadline(t *testing.T) {
	p := Timeout(TimeoutOptions{Duration: 10 * time.Millisecond})
	h := p(func(ctx context.Context) (any, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return "late", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	start := time.Now()
	got, err := h(context.Background())
	elapsed := time.Since(start)

	if got != nil {
		t.Fatalf("expected nil result, got %v", got)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected to return promptly, took %v", elapsed)
	}
}

func TestTimeout_PassesTimeoutContextToHandler(t *testing.T) {
	p := Timeout(TimeoutOptions{Duration: 50 * time.Millisecond})

	h := p(func(ctx context.Context) (any, error) {
		_, ok := ctx.Deadline()
		if !ok {
			t.Fatalf("expected ctx to have a deadline")
		}
		return "ok", nil
	})

	got, err := h(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "ok" {
		t.Fatalf("expected ok, got %v", got)
	}
}

func TestTimeout_DisabledWhenDurationNegative(t *testing.T) {
	called := false
	p := Timeout(TimeoutOptions{Duration: -1})
	h := p(func(ctx context.Context) (any, error) {
		called = true
		return 123, nil
	})

	got, err := gosentry.Execute(context.Background(), h)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != 123 {
		t.Fatalf("expected 123, got %v", got)
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
}


