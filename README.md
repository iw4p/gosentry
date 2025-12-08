# GoSentry (Sentry Golang)

![GoSentry Logo](logo.png)

GoSentry is a lightweight, composable policy-based execution framework for Go. It provides a clean middleware-style pattern for wrapping handlers with various execution policies like retry, circuit breaking, timeouts, and more.

## Architecture

GoSentry follows a simple Handler → Policy → Execute pattern:

- **Handler**: A function that performs the actual work
- **Policy**: A middleware function that wraps and enhances handlers
- **Execute**: Composes multiple policies and executes the handler

## Installation

```bash
go get gosentry
```

## Quick Start

```go
package main

import (
    "context"
    "gosentry"
    "gosentry/policies"
    "time"
)

func main() {
    // Configure retry policy
    retryOptions := policies.RetryOptions{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     10 * time.Second,
        Backoff:      policies.BackoffExponential,
        Jitter:       true,
    }
    retryPolicy := policies.Retry(retryOptions)

    // Define your handler
    handler := func(ctx context.Context) (any, error) {
        // Your business logic here
        return "result", nil
    }

    // Execute with policies
    result, err := gosentry.Execute(context.Background(), handler, retryPolicy)
    if err != nil {
        // Handle error
    }
}
```

## Policies

### Retry Policy

The retry policy automatically retries failed operations with configurable backoff strategies.

**Features:**
- Configurable max attempts
- Multiple backoff strategies (fixed, linear, exponential)
- Optional jitter to prevent thundering herd
- Context-aware cancellation

**Example:**

```go
retryOptions := policies.RetryOptions{
    MaxAttempts:  3,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     5 * time.Second,
    Backoff:      policies.BackoffExponential,
    Jitter:       true,
}
retryPolicy := policies.Retry(retryOptions)
```

**Backoff Strategies:**
- `BackoffFixed`: Constant delay between retries
- `BackoffLinear`: Linear increase in delay
- `BackoffExponential`: Exponential backoff (default)

## Composing Multiple Policies

Policies are applied in reverse order (last policy wraps first):

```go
result, err := gosentry.Execute(
    ctx,
    handler,
    timeoutPolicy,    // Applied first (outermost)
    retryPolicy,      // Applied second
    circuitBreakerPolicy, // Applied last (innermost)
)
```

## Roadmap

The following policies are implemented or planned:

- [x] **Retry** - Automatically retry failed operations with configurable backoff strategies
- [ ] **Circuit Breaker** - Prevent cascading failures by opening circuit after threshold failures
- [ ] **Timeout** - Enforce maximum execution time for handlers
- [ ] **Rate Limiting** - Control the rate of execution (token bucket, sliding window)
- [ ] **Bulkhead** - Isolate execution contexts to prevent resource exhaustion
- [ ] **Fallback** - Provide default values or alternative handlers on failure

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

