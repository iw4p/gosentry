package gosentry

import "context"

// Handler -> Policy -> Execute
type Handler func(ctx context.Context) (any, error)

type Policy func(next Handler) Handler

func Execute(ctx context.Context, handler Handler, policies ...Policy) (any, error) {
	h := handler
	for i := len(policies) - 1; i >= 0; i-- {
		h = policies[i](h)
	}
	return h(ctx)
}
