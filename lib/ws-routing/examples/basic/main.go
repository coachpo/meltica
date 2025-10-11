package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

func main() {
	ctx := context.Background()

	dialer := &demoDialer{conn: &demoConnection{}}
	parser := demoParser{}
	publish := demoPublisher

	session, err := wsrouting.Init(ctx, wsrouting.Options{
		SessionID: "example-basic",
		Dialer:    dialer,
		Parser:    parser,
		Publish:   publish,
		Backoff: wsrouting.BackoffConfig{
			Initial: 500 * time.Millisecond,
			Max:     5 * time.Second,
		},
	})
	if err != nil {
		panic(err)
	}

	_ = wsrouting.UseMiddleware(session, func(ctx context.Context, msg *wsrouting.Message) (*wsrouting.Message, error) {
		msg.Metadata["middleware"] = "invoked"
		return msg, nil
	})

	spec := wsrouting.SubscriptionSpec{Exchange: "demo", Channel: "ticker", Symbols: []string{"BTC-USD"}}
	if err := wsrouting.Subscribe(ctx, session, spec); err != nil {
		panic(err)
	}

	if err := wsrouting.Start(ctx, session); err != nil {
		panic(err)
	}

	raw := []byte(`{"type":"demo.event","payload":{"price":"42000.10"}}`)
	if err := wsrouting.RouteRaw(ctx, session, raw); err != nil {
		panic(err)
	}

	manual := &wsrouting.Message{Type: "demo.manual", Payload: map[string]any{"greeting": "hello"}, Metadata: map[string]string{}}
	if err := wsrouting.Publish(ctx, session, manual); err != nil {
		panic(err)
	}

	fmt.Println("example complete")
}

type demoDialer struct {
	conn *demoConnection
}

func (d *demoDialer) Dial(ctx context.Context, opts wsrouting.DialOptions) (wsrouting.Connection, error) {
	fmt.Printf("dialing session %s with initial backoff %s\n", opts.SessionID, opts.Backoff.Initial)
	return d.conn, nil
}

type demoConnection struct {
	subscriptions []wsrouting.SubscriptionSpec
}

func (c *demoConnection) Subscribe(_ context.Context, spec wsrouting.SubscriptionSpec) error {
	c.subscriptions = append(c.subscriptions, spec)
	fmt.Printf("subscribed to %s %s\n", spec.Channel, spec.Symbols)
	return nil
}

func (c *demoConnection) Close(context.Context) error {
	fmt.Println("connection closed")
	return nil
}

type demoParser struct{}

func (demoParser) Parse(_ context.Context, raw []byte) (*wsrouting.Message, error) {
	var envelope struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return &wsrouting.Message{
		Type:     envelope.Type,
		Payload:  envelope.Payload,
		Metadata: map[string]string{"source": "parser"},
	}, nil
}

func demoPublisher(_ context.Context, msg *wsrouting.Message) error {
	fmt.Printf("published message %s payload=%v metadata=%v\n", msg.Type, msg.Payload, msg.Metadata)
	return nil
}
