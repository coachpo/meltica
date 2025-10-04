package mocks

import (
	"context"

	coreexchange "github.com/coachpo/meltica/core/exchange"
)

// RESTClient is a test double implementing coreexchange.RESTClient.
type RESTClient struct {
	ConnectFunc        func(ctx context.Context) error
	CloseFunc          func() error
	DoRequestFunc      func(ctx context.Context, req coreexchange.RESTRequest) (*coreexchange.RESTResponse, error)
	HandleResponseFunc func(ctx context.Context, req coreexchange.RESTRequest, resp *coreexchange.RESTResponse, out any) error
	HandleErrorFunc    func(ctx context.Context, req coreexchange.RESTRequest, err error) error
}

func (m *RESTClient) Connect(ctx context.Context) error {
	if m != nil && m.ConnectFunc != nil {
		return m.ConnectFunc(ctx)
	}
	return nil
}

func (m *RESTClient) Close() error {
	if m != nil && m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *RESTClient) DoRequest(ctx context.Context, req coreexchange.RESTRequest) (*coreexchange.RESTResponse, error) {
	if m != nil && m.DoRequestFunc != nil {
		return m.DoRequestFunc(ctx, req)
	}
	return nil, nil
}

func (m *RESTClient) HandleResponse(ctx context.Context, req coreexchange.RESTRequest, resp *coreexchange.RESTResponse, out any) error {
	if m != nil && m.HandleResponseFunc != nil {
		return m.HandleResponseFunc(ctx, req, resp, out)
	}
	return nil
}

func (m *RESTClient) HandleError(ctx context.Context, req coreexchange.RESTRequest, err error) error {
	if m != nil && m.HandleErrorFunc != nil {
		return m.HandleErrorFunc(ctx, req, err)
	}
	return err
}
