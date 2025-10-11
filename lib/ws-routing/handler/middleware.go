package handler

import (
	"context"
	"fmt"
	"sync"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
)

// Middleware mutates or filters events before handlers execute.
type Middleware func(context.Context, internal.Event) (internal.Event, error)

// Chain provides ordered middleware execution.
type Chain struct {
	mu         sync.RWMutex
	middleware []Middleware
}

// NewChain builds an empty middleware chain.
func NewChain() *Chain {
	return &Chain{middleware: make([]Middleware, 0)}
}

// Use appends middleware to the execution chain.
func (c *Chain) Use(m Middleware) error {
	if m == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("middleware required"))
	}
	c.mu.Lock()
	c.middleware = append(c.middleware, m)
	c.mu.Unlock()
	return nil
}

// Apply executes the configured middleware in order.
func (c *Chain) Apply(ctx context.Context, event internal.Event) (result internal.Event, err error) {
	middlewares := c.snapshot()
	result = event
	defer func() {
		if rec := recover(); rec != nil {
			err = errs.New("", errs.CodeExchange, errs.WithMessage("middleware panic"), errs.WithCause(fmt.Errorf("%v", rec)))
		}
	}()
	for _, mw := range middlewares {
		if mw == nil {
			continue
		}
		next, mwErr := mw(ctx, result)
		if mwErr != nil {
			return internal.Event{}, mwErr
		}
		result = next
	}
	return result, nil
}

func (c *Chain) snapshot() []Middleware {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.middleware) == 0 {
		return nil
	}
	copyOf := make([]Middleware, len(c.middleware))
	copy(copyOf, c.middleware)
	return copyOf
}
