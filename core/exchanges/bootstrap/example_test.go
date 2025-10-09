package bootstrap_test

import (
	"context"
	"fmt"
	"math/big"
	"net/http"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/bootstrap"
	"github.com/coachpo/meltica/exchanges/shared/routing"
)

func ExampleOrderAmendRequestFromParts() {
	qty := big.NewRat(3, 2)
	price := big.NewRat(25_000, 1)
	req := bootstrap.OrderAmendRequestFromParts("BTC-USDT", "12345", "client-7", qty, price, core.GTC)
	fmt.Printf("%s %s %s\n", req.Symbol, req.NewQuantity.RatString(), req.NewPrice.RatString())
	// Output: BTC-USDT 3/2 25000
}

type exampleDispatcher struct{}

func (exampleDispatcher) Dispatch(_ context.Context, _ routing.RESTMessage, out any) error {
	if order, ok := out.(*core.Order); ok {
		*order = core.Order{ID: "ORD-42"}
	}
	return nil
}

type exampleTranslator struct{}

func (exampleTranslator) PrepareCreate(_ context.Context, req core.OrderRequest) (routing.DispatchSpec, error) {
	symbol := req.Symbol
	spec := routing.DispatchSpec{
		Message: routing.RESTMessage{Method: http.MethodPost, Path: "/orders"},
		Into:    &core.Order{},
		Decode: func(result any) (core.Order, error) {
			orderPtr, ok := result.(*core.Order)
			if !ok {
				return core.Order{}, fmt.Errorf("unexpected result type %T", result)
			}
			order := *orderPtr
			order.Symbol = symbol
			return order, nil
		},
	}
	return spec, nil
}

func (exampleTranslator) PrepareAmend(context.Context, routing.OrderAmendRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{}, fmt.Errorf("amend not implemented")
}

func (exampleTranslator) PrepareGet(context.Context, routing.OrderQueryRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{}, fmt.Errorf("get not implemented")
}

func (exampleTranslator) PrepareCancel(context.Context, routing.OrderCancelRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{}, fmt.Errorf("cancel not implemented")
}

func ExampleNewTradingService() {
	router, err := routing.NewOrderRouter(exampleDispatcher{}, exampleTranslator{})
	if err != nil {
		panic(err)
	}

	service, err := bootstrap.NewTradingService(bootstrap.TradingServiceConfig{Router: router})
	if err != nil {
		panic(err)
	}

	order, err := service.Place(context.Background(), core.OrderRequest{
		Symbol:      "BTC-USDT",
		Side:        core.SideBuy,
		Type:        core.TypeLimit,
		Quantity:    big.NewRat(1, 1),
		Price:       big.NewRat(20_000, 1),
		TimeInForce: core.GTC,
		ClientID:    "client-7",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(order.ID)
	fmt.Println(order.Symbol)
	fmt.Println(service.Supports(routing.ActionCancel))
	// Output:
	// ORD-42
	// BTC-USDT
	// true
}
