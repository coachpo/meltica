package errs

import (
	"strconv"
	"strings"
)

type Code string

const (
	CodeRateLimited Code = "rate_limited"
	CodeAuth        Code = "auth"
	CodeInvalid     Code = "invalid_request"
	CodeExchange    Code = "exchange_error"
	CodeNetwork     Code = "network"
)

type E struct {
	Exchange string
	Code     Code
	HTTP     int
	RawCode  string
	RawMsg   string
	Message  string

	cause error
}

type Option func(*E)

func New(exchange string, code Code, opts ...Option) *E {
	e := &E{Exchange: strings.TrimSpace(exchange), Code: code}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

func WithMessage(message string) Option {
	trimmed := strings.TrimSpace(message)
	return func(e *E) {
		e.Message = trimmed
	}
}

func WithHTTP(status int) Option {
	return func(e *E) {
		e.HTTP = status
	}
}

func WithRawCode(code string) Option {
	trimmed := strings.TrimSpace(code)
	return func(e *E) {
		e.RawCode = trimmed
	}
}

func WithRawMessage(msg string) Option {
	return func(e *E) {
		e.RawMsg = msg
	}
}

func WithCause(err error) Option {
	return func(e *E) {
		e.cause = err
	}
}

func (e *E) Error() string {
	if e == nil {
		return "<nil>"
	}
	var parts []string

	exchange := strings.TrimSpace(e.Exchange)
	if exchange == "" {
		exchange = "unknown"
	}
	parts = append(parts, "exchange="+exchange)

	code := strings.TrimSpace(string(e.Code))
	if code == "" {
		code = "unknown"
	}
	parts = append(parts, "code="+code)

	if e.HTTP > 0 {
		parts = append(parts, "http="+strconv.Itoa(e.HTTP))
	}
	if e.Message != "" {
		parts = append(parts, "message="+strconv.Quote(e.Message))
	}
	if e.RawCode != "" {
		parts = append(parts, "raw_code="+strconv.Quote(e.RawCode))
	}
	if e.RawMsg != "" {
		parts = append(parts, "raw_msg="+strconv.Quote(e.RawMsg))
	}
	if e.cause != nil {
		parts = append(parts, "cause="+strconv.Quote(e.cause.Error()))
	}

	return strings.Join(parts, " ")
}

func (e *E) Unwrap() error { return e.cause }

// NotSupported returns a standardized error for unsupported capabilities.
func NotSupported(msg string) *E {
	return New("", CodeExchange, WithMessage(strings.TrimSpace(msg)))
}
