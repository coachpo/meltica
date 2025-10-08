package errs

import (
	"sort"
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

// CanonicalCode captures exchange-agnostic error categories.
type CanonicalCode string

const (
	CanonicalUnknown             CanonicalCode = "unknown"
	CanonicalCapabilityMissing   CanonicalCode = "capability_missing"
	CanonicalOrderNotFound       CanonicalCode = "order_not_found"
	CanonicalInsufficientBalance CanonicalCode = "insufficient_balance"
	CanonicalInvalidSymbol       CanonicalCode = "invalid_symbol"
	CanonicalRateLimited         CanonicalCode = "rate_limited"
)

type E struct {
	Exchange      string
	Code          Code
	HTTP          int
	RawCode       string
	RawMsg        string
	Message       string
	Canonical     CanonicalCode
	VenueMetadata map[string]string
	Remediation   string

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

func WithRemediation(remediation string) Option {
	trimmed := strings.TrimSpace(remediation)
	return func(e *E) {
		e.Remediation = trimmed
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

// WithCanonicalCode sets the canonical error code describing the failure category.
func WithCanonicalCode(code CanonicalCode) Option {
	trimmed := strings.TrimSpace(string(code))
	return func(e *E) {
		if trimmed == "" {
			e.Canonical = CanonicalUnknown
			return
		}
		e.Canonical = CanonicalCode(trimmed)
	}
}

// WithVenueMetadata merges the provided venue metadata into the error envelope.
func WithVenueMetadata(meta map[string]string) Option {
	return func(e *E) {
		if len(meta) == 0 {
			return
		}
		if e.VenueMetadata == nil {
			e.VenueMetadata = make(map[string]string, len(meta))
		}
		for k, v := range meta {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			value := strings.TrimSpace(v)
			e.VenueMetadata[key] = value
		}
	}
}

// WithVenueField appends a single venue metadata key/value pair.
func WithVenueField(key, value string) Option {
	return func(e *E) {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return
		}
		if e.VenueMetadata == nil {
			e.VenueMetadata = make(map[string]string, 1)
		}
		e.VenueMetadata[trimmedKey] = strings.TrimSpace(value)
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

	if cc := strings.TrimSpace(string(e.Canonical)); cc != "" && cc != string(CanonicalUnknown) {
		parts = append(parts, "canonical="+cc)
	}

	if e.HTTP > 0 {
		parts = append(parts, "http="+strconv.Itoa(e.HTTP))
	}
	if e.Message != "" {
		parts = append(parts, "message="+strconv.Quote(e.Message))
	}
	if e.Remediation != "" {
		parts = append(parts, "remediation="+strconv.Quote(e.Remediation))
	}
	if e.RawCode != "" {
		parts = append(parts, "raw_code="+strconv.Quote(e.RawCode))
	}
	if e.RawMsg != "" {
		parts = append(parts, "raw_msg="+strconv.Quote(e.RawMsg))
	}
	if len(e.VenueMetadata) > 0 {
		keys := make([]string, 0, len(e.VenueMetadata))
		for k := range e.VenueMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, k := range keys {
			v := e.VenueMetadata[k]
			pairs = append(pairs, k+"="+strconv.Quote(v))
		}
		parts = append(parts, "venue="+strings.Join(pairs, ","))
	}
	if e.cause != nil {
		parts = append(parts, "cause="+strconv.Quote(e.cause.Error()))
	}

	return strings.Join(parts, " ")
}

func (e *E) Unwrap() error { return e.cause }

// NotSupported returns a standardized error for unsupported capabilities.
func NotSupported(msg string) *E {
	return New("", CodeExchange, WithMessage(strings.TrimSpace(msg)), WithCanonicalCode(CanonicalCapabilityMissing))
}
