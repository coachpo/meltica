package middleware

import (
	"context"
	"sync"

	"github.com/coachpo/meltica/errs"
)

// Handler represents a middleware function over a generic payload.
type Handler func(context.Context, any) (any, error)

// Chain maintains ordered middleware handlers.
type Chain struct {
	mu       sync.RWMutex
	handlers []Handler
}

// NewChain constructs an empty middleware chain.
func NewChain() *Chain {
	return &Chain{handlers: make([]Handler, 0)}
}

// Use appends a handler to the chain.
func (c *Chain) Use(handler Handler) error {
	if handler == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("middleware handler required"))
	}
	c.mu.Lock()
	c.handlers = append(c.handlers, handler)
	c.mu.Unlock()
	return nil
}

// Execute runs the handlers sequentially until completion or error.
func (c *Chain) Execute(ctx context.Context, input any) (any, error) {
	c.mu.RLock()
	handlers := append([]Handler(nil), c.handlers...)
	c.mu.RUnlock()
	current := input
	for _, handler := range handlers {
		var err error
		current, err = handler(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}
