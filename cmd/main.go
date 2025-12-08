package main

import (
	"context"
	"fmt"
	"gosentry"
	"net/http"
	"time"

	"gosentry/policies"
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
	handler := func(ctx context.Context) (any, error) {
		resp, err := http.Get("https://www.google.com/")
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	resp, err := gosentry.Execute(context.Background(), handler, retryPolicy)
	fmt.Println(resp)
	fmt.Println(err)
}
