package policies

import (
	"context"
	"time"

	"gosentry"
)

type TimeoutOptions struct {
	Duration time.Duration
}

func DefaultTimeoutOptions() TimeoutOptions {
	return TimeoutOptions{
		Duration: 5 * time.Second,
	}
}

func Timeout(options TimeoutOptions) gosentry.Policy {
	opts := applyTimeoutDefaults(options)

	// If disabled, return a no-op policy.
	if opts.Duration < 0 {
		return func(next gosentry.Handler) gosentry.Handler { return next }
	}

	return func(next gosentry.Handler) gosentry.Handler {
		return func(ctx context.Context) (any, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, opts.Duration)
			defer cancel()

			type outcome struct {
				result any
				err    error
			}

			done := make(chan outcome, 1)
			go func() {
				res, err := next(timeoutCtx)
				done <- outcome{result: res, err: err}
			}()

			select {
			case out := <-done:
				return out.result, out.err
			case <-timeoutCtx.Done():
				return nil, timeoutCtx.Err()
			}
		}
	}
}

func applyTimeoutDefaults(options TimeoutOptions) TimeoutOptions {
	defaults := DefaultTimeoutOptions()

	if options.Duration == 0 {
		options.Duration = defaults.Duration
	}

	return options
}
