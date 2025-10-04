package mocks

import (
	"context"

	coreexchange "github.com/coachpo/meltica/core/exchange"
)

// StreamClient is a test double implementing coreexchange.StreamClient.
type StreamClient struct {
	ConnectFunc     func(ctx context.Context) error
	CloseFunc       func() error
	SubscribeFunc   func(ctx context.Context, topics ...coreexchange.StreamTopic) (coreexchange.StreamSubscription, error)
	UnsubscribeFunc func(ctx context.Context, sub coreexchange.StreamSubscription, topics ...coreexchange.StreamTopic) error
	PublishFunc     func(ctx context.Context, message coreexchange.StreamMessage) error
	HandleErrorFunc func(ctx context.Context, err error) error
}

func (m *StreamClient) Connect(ctx context.Context) error {
	if m != nil && m.ConnectFunc != nil {
		return m.ConnectFunc(ctx)
	}
	return nil
}

func (m *StreamClient) Close() error {
	if m != nil && m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *StreamClient) Subscribe(ctx context.Context, topics ...coreexchange.StreamTopic) (coreexchange.StreamSubscription, error) {
	if m != nil && m.SubscribeFunc != nil {
		return m.SubscribeFunc(ctx, topics...)
	}
	return nil, nil
}

func (m *StreamClient) Unsubscribe(ctx context.Context, sub coreexchange.StreamSubscription, topics ...coreexchange.StreamTopic) error {
	if m != nil && m.UnsubscribeFunc != nil {
		return m.UnsubscribeFunc(ctx, sub, topics...)
	}
	if sub != nil {
		return sub.Close()
	}
	return nil
}

func (m *StreamClient) Publish(ctx context.Context, message coreexchange.StreamMessage) error {
	if m != nil && m.PublishFunc != nil {
		return m.PublishFunc(ctx, message)
	}
	return nil
}

func (m *StreamClient) HandleError(ctx context.Context, err error) error {
	if m != nil && m.HandleErrorFunc != nil {
		return m.HandleErrorFunc(ctx, err)
	}
	return err
}

// StreamSubscription is a test double implementing coreexchange.StreamSubscription.
type StreamSubscription struct {
	MessagesFunc func() <-chan coreexchange.RawMessage
	ErrorsFunc   func() <-chan error
	CloseFunc    func() error
}

func (m *StreamSubscription) Messages() <-chan coreexchange.RawMessage {
	if m != nil && m.MessagesFunc != nil {
		return m.MessagesFunc()
	}
	ch := make(chan coreexchange.RawMessage)
	close(ch)
	return ch
}

func (m *StreamSubscription) Errors() <-chan error {
	if m != nil && m.ErrorsFunc != nil {
		return m.ErrorsFunc()
	}
	ch := make(chan error)
	close(ch)
	return ch
}

func (m *StreamSubscription) Close() error {
	if m != nil && m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
