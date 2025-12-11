package policies

import (
	"context"
	"errors"
	"sync"
	"time"

	"gosentry"
)

var (
	// ErrCircuitOpen is returned when the circuit is open and calls are rejected.
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrCircuitHalfOpenBusy is returned when the circuit is half-open and a trial call is already in-flight.
	ErrCircuitHalfOpenBusy = errors.New("circuit breaker is half-open and busy")
)

type CircuitBreakerState string

const (
	CircuitClosed   CircuitBreakerState = "closed"
	CircuitOpen     CircuitBreakerState = "open"
	CircuitHalfOpen CircuitBreakerState = "half-open"
)

type CircuitBreakerOptions struct {
	// FailureThreshold is the number of consecutive failures required to open the circuit.
	FailureThreshold int

	// SuccessThreshold is the number of consecutive successes in half-open required to close the circuit.
	SuccessThreshold int

	// OpenTimeout is how long the circuit stays open before allowing a trial call (half-open).
	OpenTimeout time.Duration

	// ShouldTrip controls which errors count as failures. If nil, any non-nil error counts.
	ShouldTrip func(err error) bool

	// OnStateChange is called when the circuit changes state.
	OnStateChange func(from CircuitBreakerState, to CircuitBreakerState)

	// Now is used for time; if nil, time.Now is used.
	Now func() time.Time
}

func DefaultCircuitBreakerOptions() CircuitBreakerOptions {
	return CircuitBreakerOptions{
		FailureThreshold: 5,
		SuccessThreshold: 1,
		OpenTimeout:      30 * time.Second,
	}
}

func CircuitBreaker(options CircuitBreakerOptions) gosentry.Policy {
	opts := applyCircuitBreakerDefaults(options)
	cb := newCircuitBreaker(opts)

	return func(next gosentry.Handler) gosentry.Handler {
		return func(ctx context.Context) (any, error) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			if err := cb.beforeCall(ctx); err != nil {
				return nil, err
			}

			result, err := next(ctx)
			cb.afterCall(err)
			return result, err
		}
	}
}

type circuitBreaker struct {
	opts CircuitBreakerOptions

	mu sync.Mutex

	state        CircuitBreakerState
	openedAt     time.Time
	failures     int
	halfSuccess  int
	halfInFlight bool
}

func newCircuitBreaker(opts CircuitBreakerOptions) *circuitBreaker {
	return &circuitBreaker{
		opts:  opts,
		state: CircuitClosed,
	}
}

func (c *circuitBreaker) beforeCall(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	now := c.opts.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	switch c.state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		if now.Sub(c.openedAt) >= c.opts.OpenTimeout {
			c.transitionLocked(CircuitHalfOpen)
			// fallthrough to half-open admission
		} else {
			return ErrCircuitOpen
		}
		fallthrough

	case CircuitHalfOpen:
		if c.halfInFlight {
			return ErrCircuitHalfOpenBusy
		}
		c.halfInFlight = true
		return nil

	default:
		// Defensive: unknown state, treat as open.
		return ErrCircuitOpen
	}
}

func (c *circuitBreaker) afterCall(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == CircuitHalfOpen {
		c.halfInFlight = false
	}

	if err == nil {
		switch c.state {
		case CircuitClosed:
			c.failures = 0
		case CircuitHalfOpen:
			c.halfSuccess++
			if c.halfSuccess >= c.opts.SuccessThreshold {
				c.failures = 0
				c.halfSuccess = 0
				c.transitionLocked(CircuitClosed)
			}
		case CircuitOpen:
			// no-op; shouldn't happen since open rejects.
		}
		return
	}

	if !c.opts.ShouldTrip(err) {
		return
	}

	switch c.state {
	case CircuitClosed:
		c.failures++
		if c.failures >= c.opts.FailureThreshold {
			c.openLocked()
		}

	case CircuitHalfOpen:
		// Any failure in half-open re-opens immediately.
		c.openLocked()

	case CircuitOpen:
		// no-op
	}
}

func (c *circuitBreaker) openLocked() {
	c.failures = 0
	c.halfSuccess = 0
	c.halfInFlight = false
	c.openedAt = c.opts.Now()
	c.transitionLocked(CircuitOpen)
}

func (c *circuitBreaker) transitionLocked(to CircuitBreakerState) {
	if c.state == to {
		return
	}
	from := c.state
	c.state = to
	if c.opts.OnStateChange != nil {
		c.opts.OnStateChange(from, to)
	}
}

func applyCircuitBreakerDefaults(options CircuitBreakerOptions) CircuitBreakerOptions {
	defaults := DefaultCircuitBreakerOptions()

	if options.FailureThreshold == 0 {
		options.FailureThreshold = defaults.FailureThreshold
	}
	if options.SuccessThreshold == 0 {
		options.SuccessThreshold = defaults.SuccessThreshold
	}
	if options.OpenTimeout == 0 {
		options.OpenTimeout = defaults.OpenTimeout
	}
	if options.ShouldTrip == nil {
		options.ShouldTrip = func(err error) bool { return err != nil }
	}
	if options.Now == nil {
		options.Now = time.Now
	}

	return options
}
