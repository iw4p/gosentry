package policies

import (
	"context"
	"errors"
	"time"

	resilience "gosentry"
)

type RetryOptions struct {
	MaxAttempts int
	Delay       time.Duration
}

func Retry(retryOptions RetryOptions) resilience.Policy {
	return func(next resilience.Handler) resilience.Handler {
		return func(ctx context.Context) (any, error) {
			delay := retryOptions.Delay
			maxAttempts := retryOptions.MaxAttempts
			for i := 0; i < maxAttempts; i++ {
				resp, err := next(ctx)
				if err != nil {
					if i == maxAttempts-1 {
						return nil, err
					}
					time.Sleep(delay)
					delay *= 2
					continue
				}
				return resp, nil
			}
			return nil, errors.New("max attempts reached")
		}
	}
}
