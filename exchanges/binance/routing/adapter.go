package routing

import (
	"context"

	"github.com/coachpo/meltica/core/layers"
	"github.com/coachpo/meltica/errs"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type layerWSRouting struct {
	router  *WSRouter
	handler layers.MessageHandler
}

var _ layers.WSRouting = (*layerWSRouting)(nil)

func (l *layerWSRouting) Subscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance ws routing adapter: subscribe not implemented")
}

func (l *layerWSRouting) Unsubscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance ws routing adapter: unsubscribe not implemented")
}

func (l *layerWSRouting) OnMessage(handler layers.MessageHandler) {
	l.handler = handler
}

func (l *layerWSRouting) ParseMessage([]byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{}, errs.NotSupported("binance ws routing adapter: parse not implemented")
}

func (l *layerWSRouting) ManageListenKey(context.Context) (string, error) {
	return "", errs.NotSupported("binance ws routing adapter: manage listen key not implemented")
}

func (l *layerWSRouting) GetStreamURL(layers.SubscriptionRequest) (string, error) {
	return "", errs.NotSupported("binance ws routing adapter: stream url not implemented")
}

type layerRESTRouting struct {
	router *RESTRouter
}

var _ layers.RESTRouting = (*layerRESTRouting)(nil)

// LegacyRESTDispatcher exposes the underlying dispatcher for legacy integration points.
func (l *layerRESTRouting) LegacyRESTDispatcher() routingrest.RESTDispatcher {
	if l == nil || l.router == nil {
		return nil
	}
	return l.router.dispatcher
}

func (l *layerRESTRouting) Subscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance rest routing adapter: subscribe not implemented")
}

func (l *layerRESTRouting) Unsubscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance rest routing adapter: unsubscribe not implemented")
}

func (l *layerRESTRouting) OnMessage(layers.MessageHandler) {}

func (l *layerRESTRouting) ParseMessage([]byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{}, errs.NotSupported("binance rest routing adapter: parse not implemented")
}

func (l *layerRESTRouting) BuildRequest(context.Context, layers.APIRequest) (*layers.HTTPRequest, error) {
	return nil, errs.NotSupported("binance rest routing adapter: build request not implemented")
}

func (l *layerRESTRouting) ParseResponse(*layers.HTTPResponse) (layers.NormalizedResponse, error) {
	return layers.NormalizedResponse{}, errs.NotSupported("binance rest routing adapter: parse response not implemented")
}

type legacyRESTRouting struct {
	dispatcher routingrest.RESTDispatcher
}

var _ layers.RESTRouting = (*legacyRESTRouting)(nil)

// LegacyRESTRoutingAdapter wraps a legacy dispatcher with a layers.RESTRouting implementation.
func LegacyRESTRoutingAdapter(dispatcher routingrest.RESTDispatcher) layers.RESTRouting {
	if dispatcher == nil {
		return nil
	}
	return &legacyRESTRouting{dispatcher: dispatcher}
}

func (l *legacyRESTRouting) Subscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance legacy rest routing adapter: subscribe not implemented")
}

func (l *legacyRESTRouting) Unsubscribe(context.Context, layers.SubscriptionRequest) error {
	return errs.NotSupported("binance legacy rest routing adapter: unsubscribe not implemented")
}

func (l *legacyRESTRouting) OnMessage(layers.MessageHandler) {}

func (l *legacyRESTRouting) ParseMessage([]byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{}, errs.NotSupported("binance legacy rest routing adapter: parse not implemented")
}

func (l *legacyRESTRouting) BuildRequest(context.Context, layers.APIRequest) (*layers.HTTPRequest, error) {
	return nil, errs.NotSupported("binance legacy rest routing adapter: build request not implemented")
}

func (l *legacyRESTRouting) ParseResponse(*layers.HTTPResponse) (layers.NormalizedResponse, error) {
	return layers.NormalizedResponse{}, errs.NotSupported("binance legacy rest routing adapter: parse response not implemented")
}

// LegacyRESTDispatcher returns the wrapped dispatcher for migration use.
func (l *legacyRESTRouting) LegacyRESTDispatcher() routingrest.RESTDispatcher {
	return l.dispatcher
}
