package architecture

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/layers"
)

const contractDeadline = 100 * time.Millisecond

type connectionContractCase struct {
	name         string
	factory      func() layers.Connection
	expectIssues []string
}

func TestConnectionContract(t *testing.T) {
	cases := []connectionContractCase{
		{
			name:    "compliant",
			factory: func() layers.Connection { return &compliantConnection{} },
		},
		{
			name: "non-idempotent",
			factory: func() layers.Connection {
				return &nonIdempotentConnection{}
			},
			expectIssues: []string{
				"Connect must be idempotent",
				"Close must be safe",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			issues := runConnectionContract(t, tc.factory)
			assertRuntime(t, start, "connection contract")
			assertIssues(t, issues, tc.expectIssues)
		})
	}
}

type routingContractCase struct {
	name         string
	factory      func() layers.Routing
	expectIssues []string
}

func TestRoutingContract(t *testing.T) {
	cases := []routingContractCase{
		{
			name:    "compliant",
			factory: func() layers.Routing { return newCompliantRouting() },
		},
		{
			name:    "non-idempotent",
			factory: func() layers.Routing { return newNonIdempotentRouting() },
			expectIssues: []string{
				"Subscribe must be idempotent",
				"Unsubscribe must be safe",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			issues := runRoutingContract(t, tc.factory)
			assertRuntime(t, start, "routing contract")
			assertIssues(t, issues, tc.expectIssues)
		})
	}
}

type businessContractCase struct {
	name         string
	factory      func() layers.Business
	expectIssues []string
}

func TestBusinessContract(t *testing.T) {
	cases := []businessContractCase{
		{
			name:    "compliant",
			factory: func() layers.Business { return &compliantBusinessContract{} },
		},
		{
			name:    "non-compliant",
			factory: func() layers.Business { return &nonCompliantBusiness{} },
			expectIssues: []string{
				"Process must succeed",
				"Validate must succeed",
				"GetState must provide status",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			issues := runBusinessContract(t, tc.factory)
			assertRuntime(t, start, "business contract")
			assertIssues(t, issues, tc.expectIssues)
		})
	}
}

type filterContractCase struct {
	name         string
	factory      func() layers.Filter
	expectIssues []string
}

func TestFilterContract(t *testing.T) {
	cases := []filterContractCase{
		{
			name:    "compliant",
			factory: func() layers.Filter { return &compliantFilterContract{} },
		},
		{
			name:    "non-compliant",
			factory: func() layers.Filter { return &nonCompliantFilter{} },
			expectIssues: []string{
				"Apply must return output channel",
				"Apply must emit events",
				"Close must be safe",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			issues := runFilterContract(t, tc.factory)
			assertRuntime(t, start, "filter contract")
			assertIssues(t, issues, tc.expectIssues)
		})
	}
}

func runConnectionContract(t *testing.T, factory func() layers.Connection) []string {
	conn := factory()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var issues []string

	if err := conn.Connect(ctx); err != nil {
		issues = append(issues, "Connect must succeed")
	}
	if err := conn.Connect(ctx); err != nil {
		issues = append(issues, "Connect must be idempotent")
	}

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		issues = append(issues, "SetReadDeadline must accept future time")
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		issues = append(issues, "SetReadDeadline must clear deadline")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		issues = append(issues, "SetWriteDeadline must accept future time")
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		issues = append(issues, "SetWriteDeadline must clear deadline")
	}

	if err := conn.Close(); err != nil {
		issues = append(issues, "Close must succeed")
	}
	if err := conn.Close(); err != nil {
		issues = append(issues, "Close must be safe")
	}

	return issues
}

func runRoutingContract(t *testing.T, factory func() layers.Routing) []string {
	r := factory()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	request := layers.SubscriptionRequest{Symbol: "BTCUSDT", Type: "book"}
	var issues []string

	if err := r.Subscribe(ctx, request); err != nil {
		issues = append(issues, "Subscribe must succeed")
	}
	if err := r.Subscribe(ctx, request); err != nil {
		issues = append(issues, "Subscribe must be idempotent")
	}

	if err := r.Unsubscribe(ctx, request); err != nil {
		issues = append(issues, "Unsubscribe must succeed")
	}
	if err := r.Unsubscribe(ctx, request); err != nil {
		issues = append(issues, "Unsubscribe must be safe")
	}

	var received []layers.NormalizedMessage
	r.OnMessage(func(msg layers.NormalizedMessage) {
		received = append(received, msg)
	})

	if emitter, ok := r.(interface {
		EmitTestMessage(layers.NormalizedMessage)
	}); ok {
		emitter.EmitTestMessage(layers.NormalizedMessage{Symbol: "BTCUSDT", Type: "book"})
		if len(received) != 1 {
			issues = append(issues, "OnMessage handler must receive emitted messages")
		}
	}

	return issues
}

func assertIssues(t *testing.T, actual, expected []string) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf("unexpected issue count: got %d want %d (%v)", len(actual), len(expected), actual)
	}

	issueMap := make(map[string]struct{}, len(expected))
	for _, issue := range expected {
		issueMap[issue] = struct{}{}
	}

	for _, issue := range actual {
		if _, ok := issueMap[issue]; !ok {
			t.Fatalf("unexpected issue: %s (expected %v)", issue, expected)
		}
	}
}

func assertRuntime(t *testing.T, start time.Time, label string) {
	if d := time.Since(start); d > contractDeadline {
		t.Fatalf("%s exceeded %s (took %s)", label, contractDeadline, d)
	}
}

type compliantConnection struct{}

func (c *compliantConnection) Connect(context.Context) error { return nil }
func (c *compliantConnection) Close() error                  { return nil }
func (c *compliantConnection) IsConnected() bool             { return true }
func (c *compliantConnection) SetReadDeadline(time.Time) error {
	return nil
}
func (c *compliantConnection) SetWriteDeadline(time.Time) error {
	return nil
}

type nonIdempotentConnection struct {
	connected bool
}

func (c *nonIdempotentConnection) Connect(context.Context) error {
	if c.connected {
		return errors.New("already connected")
	}
	c.connected = true
	return nil
}

func (c *nonIdempotentConnection) Close() error {
	if !c.connected {
		return errors.New("not connected")
	}
	c.connected = false
	return nil
}

func (c *nonIdempotentConnection) IsConnected() bool { return c.connected }

func (c *nonIdempotentConnection) SetReadDeadline(time.Time) error  { return nil }
func (c *nonIdempotentConnection) SetWriteDeadline(time.Time) error { return nil }

type compliantRouting struct {
	subs map[layers.SubscriptionRequest]int
	fn   func(layers.NormalizedMessage)
}

func newCompliantRouting() *compliantRouting {
	return &compliantRouting{subs: make(map[layers.SubscriptionRequest]int)}
}

func (r *compliantRouting) Subscribe(_ context.Context, req layers.SubscriptionRequest) error {
	r.subs[req]++
	return nil
}

func (r *compliantRouting) Unsubscribe(_ context.Context, req layers.SubscriptionRequest) error {
	if _, ok := r.subs[req]; ok {
		delete(r.subs, req)
	}
	return nil
}

func (r *compliantRouting) OnMessage(fn layers.MessageHandler) { r.fn = fn }

func (r *compliantRouting) ParseMessage([]byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{Type: "book"}, nil
}

func (r *compliantRouting) EmitTestMessage(msg layers.NormalizedMessage) {
	if r.fn != nil {
		r.fn(msg)
	}
}

type nonIdempotentRouting struct {
	subs map[layers.SubscriptionRequest]bool
	fn   func(layers.NormalizedMessage)
}

func newNonIdempotentRouting() *nonIdempotentRouting {
	return &nonIdempotentRouting{subs: make(map[layers.SubscriptionRequest]bool)}
}

func (r *nonIdempotentRouting) Subscribe(_ context.Context, req layers.SubscriptionRequest) error {
	if r.subs[req] {
		return errors.New("already subscribed")
	}
	r.subs[req] = true
	return nil
}

func (r *nonIdempotentRouting) Unsubscribe(_ context.Context, req layers.SubscriptionRequest) error {
	if !r.subs[req] {
		return errors.New("not subscribed")
	}
	delete(r.subs, req)
	return nil
}

func (r *nonIdempotentRouting) OnMessage(fn layers.MessageHandler) { r.fn = fn }

func (r *nonIdempotentRouting) ParseMessage([]byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{Type: "book"}, nil
}

func (r *nonIdempotentRouting) EmitTestMessage(msg layers.NormalizedMessage) {
	if r.fn != nil {
		r.fn(msg)
	}
}

func runBusinessContract(t *testing.T, factory func() layers.Business) []string {
	biz := factory()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var issues []string

	result, err := biz.Process(ctx, layers.NormalizedMessage{Type: "contract"})
	if err != nil {
		issues = append(issues, "Process must succeed")
	} else if !result.Success {
		issues = append(issues, "Process must succeed")
	}

	if err := biz.Validate(ctx, layers.BusinessRequest{Type: "contract"}); err != nil {
		issues = append(issues, "Validate must succeed")
	}

	state := biz.GetState()
	if state.Status == "" {
		issues = append(issues, "GetState must provide status")
	}

	return issues
}

func runFilterContract(t *testing.T, factory func() layers.Filter) []string {
	filter := factory()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var issues []string
	input := make(chan layers.Event, 1)
	input <- layers.Event{Type: "trade"}
	close(input)

	output, err := filter.Apply(ctx, input)
	if err != nil || output == nil {
		issues = append(issues, "Apply must return output channel")
		issues = append(issues, "Apply must emit events")
		closeErr1 := filter.Close()
		closeErr2 := filter.Close()
		if closeErr1 != nil || closeErr2 != nil {
			issues = append(issues, "Close must be safe")
		}
		return issues
	}

	select {
	case _, ok := <-output:
		if !ok {
			issues = append(issues, "Apply must emit events")
		}
	case <-ctx.Done():
		issues = append(issues, "Apply must emit events")
	}

	closeErr1 := filter.Close()
	closeErr2 := filter.Close()
	if closeErr1 != nil || closeErr2 != nil {
		issues = append(issues, "Close must be safe")
	}

	return issues
}

type compliantBusinessContract struct{}

func (c *compliantBusinessContract) Process(context.Context, layers.NormalizedMessage) (layers.BusinessResult, error) {
	return layers.BusinessResult{Success: true}, nil
}

func (c *compliantBusinessContract) Validate(context.Context, layers.BusinessRequest) error {
	return nil
}

func (c *compliantBusinessContract) GetState() layers.BusinessState {
	return layers.BusinessState{Status: "ready", Metrics: map[string]any{"calls": 1}, LastUpdate: time.Now().UTC()}
}

type nonCompliantBusiness struct{}

func (n *nonCompliantBusiness) Process(context.Context, layers.NormalizedMessage) (layers.BusinessResult, error) {
	return layers.BusinessResult{}, errors.New("process failed")
}

func (n *nonCompliantBusiness) Validate(context.Context, layers.BusinessRequest) error {
	return errors.New("validate failed")
}

func (n *nonCompliantBusiness) GetState() layers.BusinessState { return layers.BusinessState{} }

type compliantFilterContract struct{}

func (c *compliantFilterContract) Apply(ctx context.Context, events <-chan layers.Event) (<-chan layers.Event, error) {
	output := make(chan layers.Event, 1)
	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				output <- evt
			}
		}
	}()
	return output, nil
}

func (c *compliantFilterContract) Name() string { return "compliant_filter" }

func (c *compliantFilterContract) Close() error { return nil }

type nonCompliantFilter struct{}

func (n *nonCompliantFilter) Apply(context.Context, <-chan layers.Event) (<-chan layers.Event, error) {
	return nil, errors.New("apply failed")
}

func (n *nonCompliantFilter) Name() string { return "non_compliant_filter" }

func (n *nonCompliantFilter) Close() error { return errors.New("close failed") }
