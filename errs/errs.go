package errs

import "fmt"

type Code string

const (
	CodeRateLimited Code = "rate_limited"
	CodeAuth        Code = "auth"
	CodeInvalid     Code = "invalid_request"
	CodeExchange    Code = "exchange_error"
	CodeNetwork     Code = "network"
)

type E struct {
	Provider string
	Code     Code
	HTTP     int
	RawCode  string
	RawMsg   string
}

func (e *E) Error() string {
	return fmt.Sprintf("%s(%s) http=%d raw=%s %s", e.Provider, e.Code, e.HTTP, e.RawCode, e.RawMsg)
}

// NotSupported returns a standardized error for unsupported capabilities.
func NotSupported(msg string) *E {
	return &E{Code: CodeExchange, RawMsg: msg}
}
