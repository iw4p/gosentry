package main

import (
	"context"
	"fmt"
	"gosentry/policies"
	"net/http"
	"time"
)

func main() {
	retryOptions := policies.RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Backoff:      policies.BackoffExponential,
		Jitter:       true,
	}
	retryPolicy := policies.Retry(retryOptions)
	wrapped := retryPolicy(func(ctx context.Context) (any, error) {
		resp, err := http.Get("https://www.google.com/")
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	resp, err := wrapped(context.Background())
	fmt.Println(resp)
	fmt.Println(err)
}
