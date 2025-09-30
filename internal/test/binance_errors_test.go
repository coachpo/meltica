package test

import (
	"testing"

	"github.com/yourorg/meltica/errs"
	"github.com/yourorg/meltica/providers/binance"
)

func Test_Binance_ErrorMapping_PreciseCodes(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   errs.Code
	}{
		{429, `{"code":-1003,"msg":"Too many requests"}`, errs.CodeRateLimited},
		{400, `{"code":-2015,"msg":"Invalid API-key, IP, or permissions"}`, errs.CodeAuth},
		{400, `{"code":-1021,"msg":"Timestamp for this request is outside of the recvWindow"}`, errs.CodeAuth},
		{400, `{"code":-1022,"msg":"Signature for this request is not valid."}`, errs.CodeAuth},
		{400, `{"code":-1013,"msg":"Invalid quantity"}`, errs.CodeInvalid},
		{400, `{"code":-1102,"msg":"Mandatory parameter was not sent"}`, errs.CodeInvalid},
		{400, `{"code":-1131,"msg":"This listenKey does not exist"}`, errs.CodeInvalid},
		{400, `{"code":-2019,"msg":"Margin is insufficient"}`, errs.CodeInvalid},
		{400, `{"code":-2022,"msg":"ReduceOnly order rejected"}`, errs.CodeInvalid},
		{500, `{"code":-1006,"msg":"Internal error"}`, errs.CodeExchange},
	}
	for _, c := range cases {
		err := binance.TestOnly_MapHTTPError(c.status, []byte(c.body))
		if err == nil {
			t.Fatalf("expected error for status %d body %s", c.status, c.body)
		}
		e, ok := err.(*errs.E)
		if !ok {
			t.Fatalf("unexpected error type %T", err)
		}
		if e.Code != c.want {
			t.Fatalf("want code %s got %s for body=%s", c.want, e.Code, c.body)
		}

		// HTTP 418 -> rate limited
		if err := binance.TestOnly_MapHTTPError(418, []byte("IP banned")); err != nil {
			if e, ok := err.(*errs.E); ok {
				if e.Code != errs.CodeRateLimited {
					t.Fatalf("HTTP 418 should map to rate_limited, got %s", e.Code)
				}
			} else {
				t.Fatalf("unexpected error type %T", err)
			}
		} else {
			t.Fatal("expected error for HTTP 418")
		}
	}
}
