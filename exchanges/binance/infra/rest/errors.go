package rest

import (
	"encoding/json"
	"fmt"

	"github.com/coachpo/meltica/errs"
)

// mapHTTPError converts Binance REST error payloads into the shared error type.
func mapHTTPError(status int, body []byte) error {
	if status == 418 || status == 429 {
		return errs.New(
			"binance",
			errs.CodeRateLimited,
			errs.WithHTTP(status),
			errs.WithMessage("binance rest rate limited"),
			errs.WithRawMessage(string(body)),
		)
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(body, &env) == nil && env.Code != 0 {
		code := errs.CodeExchange
		switch env.Code {
		case -2015, -2014, -2010, -1022, -1021, -2008, -2009:
			code = errs.CodeAuth
		case -1102, -1100, -1013, -1131, -1130, -1106, -1012, -1011, -2011, -2013, -2019, -2021, -2022:
			code = errs.CodeInvalid
		case -1003, -1125, -1015, -1016:
			code = errs.CodeRateLimited
		default:
			code = errs.CodeExchange
		}
		return errs.New(
			"binance",
			code,
			errs.WithHTTP(status),
			errs.WithMessage(fmt.Sprintf("binance rest error code=%d", env.Code)),
			errs.WithRawCode(jsonInt(env.Code)),
			errs.WithRawMessage(env.Msg),
		)
	}
	return errs.New(
		"binance",
		errs.CodeExchange,
		errs.WithHTTP(status),
		errs.WithMessage(fmt.Sprintf("binance rest http %d", status)),
		errs.WithRawMessage(string(body)),
	)
}

func jsonInt(i int) string { return fmt.Sprintf("%d", i) }
