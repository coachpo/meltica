package rest

import (
	"encoding/json"
	"fmt"

	"github.com/coachpo/meltica/errs"
)

// MapError normalizes provider-specific errors to *errs.E
func MapError(providerCode string, message string, httpStatus int) *errs.E {
	return &errs.E{
		Provider: "coinbase",
		Code:     errs.CodeExchange,
		HTTP:     httpStatus,
		RawCode:  providerCode,
		RawMsg:   message,
	}
}

func mapHTTPError(status int, body []byte) error {
	var env struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(body, &env); err == nil && (env.Message != "" || env.Type != "") {
		return MapError(env.Type, env.Message, status)
	}
	return fmt.Errorf("coinbase http %d: %s", status, string(body))
}
