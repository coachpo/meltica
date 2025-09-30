package kraken

import (
	"encoding/json"

	"github.com/coachpo/meltica/errs"
)

// mapHTTPError normalizes Kraken REST error envelopes.
func mapHTTPError(status int, body []byte) error {
	var env struct {
		Error []string `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return &errs.E{Provider: "kraken", Code: errs.CodeExchange, HTTP: status, RawMsg: string(body)}
	}
	code := errs.CodeExchange
	rawCode := ""
	rawMsg := ""
	if len(env.Error) > 0 {
		rawMsg = env.Error[0]
		rawCode = env.Error[0]
		switch {
		case hasAnyPrefix(env.Error, "EAPI:Invalid", "EGeneral:Invalid"):
			code = errs.CodeInvalid
		case hasAnyPrefix(env.Error, "EAuth:", "EService:Unavailable"):
			code = errs.CodeAuth
		case hasAnyPrefix(env.Error, "EGeneral:Rate", "EAPI:Rate"):
			code = errs.CodeRateLimited
		}
	}
	return &errs.E{Provider: "kraken", Code: code, HTTP: status, RawCode: rawCode, RawMsg: rawMsg}
}

func hasAnyPrefix(errsSlice []string, prefixes ...string) bool {
	for _, e := range errsSlice {
		for _, p := range prefixes {
			if len(e) >= len(p) && e[:len(p)] == p {
				return true
			}
		}
	}
	return false
}

func resultError(errsSlice []string) error {
	if len(errsSlice) == 0 {
		return nil
	}
	code := errs.CodeExchange
	if hasAnyPrefix(errsSlice, "EAPI:Invalid", "EGeneral:Invalid") {
		code = errs.CodeInvalid
	} else if hasAnyPrefix(errsSlice, "EAuth:", "EService:Unavailable") {
		code = errs.CodeAuth
	} else if hasAnyPrefix(errsSlice, "EGeneral:Rate", "EAPI:Rate") {
		code = errs.CodeRateLimited
	}
	return &errs.E{Provider: "kraken", Code: code, RawCode: errsSlice[0], RawMsg: errsSlice[0]}
}
