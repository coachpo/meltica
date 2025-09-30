package binance

import (
	"encoding/json"
	"fmt"

	"github.com/yourorg/meltica/errs"
)

// Map Binance error body to unified error
func mapHTTPError(status int, body []byte) error {
	if status == 418 || status == 429 {
		return &errs.E{Provider: "binance", Code: errs.CodeRateLimited, HTTP: status, RawMsg: string(body)}
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &env) == nil && env.Code != 0 {
		code := errs.CodeExchange
		switch env.Code {
		// Auth/signature/permissions/time
		case -2015, -2014, -2010, -1022, -1021, -2008, -2009:
			code = errs.CodeAuth
		// Invalid request/parameters/business constraints
		case -1102, -1100, -1013, -1131, -1130, -1106, -1012, -1011, -2011, -2013, -2019, -2021, -2022:
			code = errs.CodeInvalid
		// Rate limiting / request weight
		case -1003, -1125, -1015, -1016:
			code = errs.CodeRateLimited
		default:
			code = errs.CodeExchange
		}
		return &errs.E{Provider: "binance", Code: code, HTTP: status, RawCode: jsonInt(env.Code), RawMsg: env.Msg}
	}
	return &errs.E{Provider: "binance", Code: errs.CodeExchange, HTTP: status, RawMsg: string(body)}
}

func jsonInt(i int) string { return fmt.Sprintf("%d", i) }

// TestOnly_MapHTTPError exposes mapping for unit tests
func TestOnly_MapHTTPError(status int, body []byte) error { return mapHTTPError(status, body) }
