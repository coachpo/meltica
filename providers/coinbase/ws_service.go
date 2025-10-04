package coinbase

import (
	"context"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/providers/coinbase/routing"
)

type wsService struct {
	router *routing.WSRouter
}

func newWSService(router *routing.WSRouter) core.WS {
	return &wsService{router: router}
}

func (w *wsService) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	sub, err := w.router.SubscribePublic(ctx, topics...)
	if err != nil {
		return nil, err
	}
	return newWSServerSubscription(sub), nil
}

func (w *wsService) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	sub, err := w.router.SubscribePrivate(ctx)
	if err != nil {
		return nil, err
	}
	return newWSServerSubscription(sub), nil
}

func (w *wsService) WSNativeSymbol(canonical string) string {
	return w.router.WSNativeSymbol(canonical)
}

func (w *wsService) WSCanonicalSymbol(native string) string {
	return w.router.WSCanonicalSymbol(native)
}

type wsServerSubscription struct {
	inner routing.Subscription
	c     chan core.Message
	err   chan error
}

func newWSServerSubscription(inner routing.Subscription) *wsServerSubscription {
	s := &wsServerSubscription{
		inner: inner,
		c:     make(chan core.Message, 1024),
		err:   make(chan error, 1),
	}
	go s.forward()
	return s
}

func (s *wsServerSubscription) forward() {
	defer close(s.c)
	defer close(s.err)

	routed := s.inner.C()
	errs := s.inner.Err()

	for {
		select {
		case msg, ok := <-routed:
			if !ok {
				return
			}
			s.c <- core.Message{Topic: msg.Topic, Raw: msg.Raw, At: msg.At, Event: msg.Route, Parsed: msg.Parsed}
		case err, ok := <-errs:
			if !ok {
				return
			}
			if err != nil {
				select {
				case s.err <- err:
				default:
				}
			}
			return
		}
	}
}

func (s *wsServerSubscription) C() <-chan core.Message { return s.c }
func (s *wsServerSubscription) Err() <-chan error      { return s.err }

func (s *wsServerSubscription) Close() error {
	return s.inner.Close()
}
