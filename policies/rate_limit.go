package policies

import (
	"context"
	"errors"
	"sync"
	"time"

	"gosentry"
)

var (
	// ErrRateLimitExceeded is returned when the rate limit is reached.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type RateLimitOptions struct {
	// Rate is the number of tokens to add per second.
	Rate float64

	// Burst is the maximum number of tokens that can be stored in the bucket.
	Burst int

	// Now is used for time; if nil, time.Now is used.
	Now func() time.Time
}

func DefaultRateLimitOptions() RateLimitOptions {
	return RateLimitOptions{
		Rate:  10,
		Burst: 10,
	}
}

func RateLimit(options RateLimitOptions) gosentry.Policy {
	opts := applyRateLimitDefaults(options)

	var mu sync.Mutex
	tokens := float64(opts.Burst)
	lastAt := opts.Now()

	allow := func() bool {
		mu.Lock()
		defer mu.Unlock()

		now := opts.Now()
		elapsed := now.Sub(lastAt).Seconds()
		tokens += elapsed * opts.Rate

		if tokens > float64(opts.Burst) {
			tokens = float64(opts.Burst)
		}

		lastAt = now

		if tokens >= 1 {
			tokens -= 1
			return true
		}

		return false
	}

	return func(next gosentry.Handler) gosentry.Handler {
		return func(ctx context.Context) (any, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			if !allow() {
				return nil, ErrRateLimitExceeded
			}

			return next(ctx)
		}
	}
}

func applyRateLimitDefaults(options RateLimitOptions) RateLimitOptions {
	defaults := DefaultRateLimitOptions()

	if options.Rate <= 0 {
		options.Rate = defaults.Rate
	}
	if options.Burst <= 0 {
		options.Burst = defaults.Burst
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	return options
}
