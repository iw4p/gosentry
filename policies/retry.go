package policies

import (
	"context"
	"math/rand"
	"time"

	resilience "gosentry"
)

type BackoffStrategy string

const (
	BackoffFixed       BackoffStrategy = "fixed"
	BackoffLinear      BackoffStrategy = "linear"
	BackoffExponential BackoffStrategy = "exponential"
)

type RetryOptions struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Backoff      BackoffStrategy
	Jitter       bool
}

func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Backoff:      BackoffExponential,
		Jitter:       true,
	}
}

func Retry(options RetryOptions) resilience.Policy {
	opts := applyDefaults(options)
	return func(next resilience.Handler) resilience.Handler {
		return func(ctx context.Context) (any, error) {
			var lastErr error

			for attempt := 0; attempt < opts.MaxAttempts; attempt++ {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}

				result, err := next(ctx)
				if err == nil {
					return result, nil
				}

				lastErr = err
				if attempt == opts.MaxAttempts-1 {
					break
				}

				delay := computeDelay(attempt, opts)
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
					timer.Stop()
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				}
			}

			return nil, lastErr
		}
	}
}

func computeDelay(attempt int, opts RetryOptions) time.Duration {
	var delay time.Duration

	switch opts.Backoff {
	case BackoffFixed:
		delay = opts.InitialDelay
	case BackoffLinear:
		delay = opts.InitialDelay * time.Duration(attempt+1)
	case BackoffExponential:
		delay = opts.InitialDelay * time.Duration(1<<uint(attempt))
	default:
		delay = opts.InitialDelay
	}

	if opts.Jitter {
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		delay += jitter
	}

	if delay > opts.MaxDelay {
		delay = opts.MaxDelay
	}

	return delay
}

func applyDefaults(options RetryOptions) RetryOptions {
	defaults := DefaultRetryOptions()

	if options.MaxAttempts == 0 {
		options.MaxAttempts = defaults.MaxAttempts
	}
	if options.InitialDelay == 0 {
		options.InitialDelay = defaults.InitialDelay
	}
	if options.MaxDelay == 0 {
		options.MaxDelay = defaults.MaxDelay
	}
	if options.Backoff == "" {
		options.Backoff = defaults.Backoff
	}

	return options
}
