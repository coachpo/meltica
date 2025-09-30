package coinbase

import "github.com/yourorg/meltica/errs"

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
