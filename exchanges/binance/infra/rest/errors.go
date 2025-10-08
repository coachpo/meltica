package rest

import (
	"encoding/json"
	"fmt"
	"strings"

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
			errs.WithCanonicalCode(errs.CanonicalRateLimited),
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
		options := []errs.Option{
			errs.WithHTTP(status),
			errs.WithMessage(fmt.Sprintf("binance rest error code=%d", env.Code)),
			errs.WithRawCode(jsonInt(env.Code)),
			errs.WithRawMessage(env.Msg),
		}
		if canonical, remediation := canonicalForBinance(env.Code, env.Msg); canonical != errs.CanonicalUnknown {
			options = append(options, errs.WithCanonicalCode(canonical))
			if remediation != "" {
				options = append(options, errs.WithRemediation(remediation))
			}
		}
		return errs.New(
			"binance",
			code,
			options...,
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

func canonicalForBinance(code int, message string) (errs.CanonicalCode, string) {
	switch code {
	case -2011, -2013, -2022:
		return errs.CanonicalOrderNotFound, ""
	case -2010:
		if lookLower(message, "insufficient") || lookLower(message, "not enough") {
			return errs.CanonicalInsufficientBalance, ""
		}
	case -1121, -1130:
		return errs.CanonicalInvalidSymbol, ""
	}
	return errs.CanonicalUnknown, ""
}

func lookLower(message, needle string) bool {
	if needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(message), strings.ToLower(needle))
}
