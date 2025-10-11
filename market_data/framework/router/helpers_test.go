package router

import (
	"context"
	"sync"

	"github.com/coachpo/meltica/errs"
)

type stubDialer struct {
	mu      sync.Mutex
	conn    *stubConnection
	calls   int
	lastCtx context.Context
	err     error
}

func (d *stubDialer) Dial(ctx context.Context, opts DialOptions) (Connection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.lastCtx = ctx
	if d.err != nil {
		return nil, d.err
	}
	if d.conn == nil {
		d.conn = &stubConnection{}
	}
	d.conn.mu.Lock()
	d.conn.lastDial = opts
	d.conn.mu.Unlock()
	return d.conn, nil
}

func (d *stubDialer) Calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

type stubConnection struct {
	mu            sync.Mutex
	subscriptions []SubscriptionSpec
	lastDial      DialOptions
}

func (c *stubConnection) Subscribe(_ context.Context, spec SubscriptionSpec) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions = append(c.subscriptions, spec)
	return nil
}

func (c *stubConnection) Close(context.Context) error { return nil }

func (c *stubConnection) Subscriptions() []SubscriptionSpec {
	c.mu.Lock()
	defer c.mu.Unlock()
	clone := make([]SubscriptionSpec, len(c.subscriptions))
	copy(clone, c.subscriptions)
	return clone
}

type stubParser struct {
	mu  sync.Mutex
	raw [][]byte
	msg *Message
	err error
}

func (p *stubParser) Parse(_ context.Context, raw []byte) (*Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.raw = append(p.raw, append([]byte(nil), raw...))
	if p.err != nil {
		return nil, p.err
	}
	if p.msg != nil {
		clone := *p.msg
		if clone.Metadata != nil {
			copyMeta := make(map[string]string, len(clone.Metadata))
			for k, v := range clone.Metadata {
				copyMeta[k] = v
			}
			clone.Metadata = copyMeta
		}
		return &clone, nil
	}
	return &Message{Type: "stub", Payload: map[string]any{"seen": string(raw)}, Metadata: map[string]string{}}, nil
}

type stubPublisher struct {
	mu      sync.Mutex
	msgs    []*Message
	invoked int
	err     error
}

func (p *stubPublisher) Publish(_ context.Context, msg *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invoked++
	if msg != nil {
		clone := *msg
		if clone.Metadata != nil {
			meta := make(map[string]string, len(clone.Metadata))
			for k, v := range clone.Metadata {
				meta[k] = v
			}
			clone.Metadata = meta
		}
		p.msgs = append(p.msgs, &clone)
	}
	if p.err != nil {
		return p.err
	}
	return nil
}

func (p *stubPublisher) Messages() []*Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*Message(nil), p.msgs...)
}

func (p *stubPublisher) Invocations() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.invoked
}

type middlewareRecorder struct {
	mu      sync.Mutex
	calls   int
	lastCtx context.Context
}

func (m *middlewareRecorder) Handler(ctx context.Context, msg *Message) (*Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastCtx = ctx
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}
	msg.Metadata["middleware"] = "seen"
	return msg, nil
}

type errMiddleware struct{}

func (errMiddleware) Handler(context.Context, *Message) (*Message, error) {
	return nil, errs.New("", errs.CodeExchange, errs.WithMessage("middleware failure"))
}

type errParser struct{}

func (errParser) Parse(context.Context, []byte) (*Message, error) {
	return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("parse failure"))
}
