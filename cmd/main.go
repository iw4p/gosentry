package main

import (
	"context"
	"fmt"
	resilience "gosentry"
	"gosentry/policies"
	"net/http"
	"time"
)

func main() {
	resp, err := resilience.Execute(context.Background(), func(ctx context.Context) (any, error) {
		resp, err := http.Get("http://example.com/")
		if err != nil {
			return nil, err
		}
		return resp, nil
	}, policies.Retry(policies.RetryOptions{
		MaxAttempts: 3,
		Delay:       1 * time.Second,
	}))
	fmt.Println(resp)
	fmt.Println(err)
}
