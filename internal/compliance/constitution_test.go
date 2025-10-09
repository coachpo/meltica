package compliance_test

import (
	"reflect"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/market_data"
)

func TestConstitution_CoreStructsAvoidFloats(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(core.OrderRequest{}),
		reflect.TypeOf(core.Order{}),
		reflect.TypeOf(core.Book{}),
		reflect.TypeOf(core.Ticker{}),
		reflect.TypeOf(core.Position{}),
		reflect.TypeOf(market_data.TradePayload{}),
		reflect.TypeOf(market_data.OrderBookPayload{}),
		reflect.TypeOf(market_data.AccountPayload{}),
	}
	visited := make(map[reflect.Type]bool)
	for _, typ := range types {
		ensureNoFloatFields(t, typ, visited)
	}
}

func ensureNoFloatFields(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	if typ.Kind() != reflect.Struct {
		return
	}
	if visited[typ] {
		return
	}
	visited[typ] = true
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		ft := field.Type
		switch ft.Kind() {
		case reflect.Float32, reflect.Float64:
			t.Fatalf("field %s in %s uses forbidden float type %s", field.Name, typ, ft.Kind())
		case reflect.Struct:
			ensureNoFloatFields(t, ft, visited)
		case reflect.Ptr, reflect.Slice, reflect.Array:
			elem := ft.Elem()
			if elem.Kind() == reflect.Struct {
				ensureNoFloatFields(t, elem, visited)
			}
		}
	}
}
