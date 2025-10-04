package internal

import (
	"fmt"

	"github.com/coachpo/meltica/errs"
)

const exchange = "binance"

// New constructs a canonical Binance error with the provided code and message.
func New(code errs.Code, msg string) *errs.E {
	return errs.New(exchange, code, errs.WithMessage(msg))
}

// Newf formats a message and constructs a canonical Binance error.
func Newf(code errs.Code, format string, args ...any) *errs.E {
	return New(code, fmt.Sprintf(format, args...))
}

// Wrap attaches an underlying error as context while preserving canonical metadata.
func Wrap(code errs.Code, err error, format string, args ...any) *errs.E {
	msg := fmt.Sprintf(format, args...)
	if err != nil {
		msg = fmt.Sprintf("%s: %v", msg, err)
		return errs.New(exchange, code, errs.WithMessage(msg), errs.WithCause(err))
	}
	return errs.New(exchange, code, errs.WithMessage(msg))
}

// Invalid reports an invalid request sent to Binance.
func Invalid(format string, args ...any) *errs.E {
	return Newf(errs.CodeInvalid, format, args...)
}

// Exchange reports a generic exchange-side failure.
func Exchange(format string, args ...any) *errs.E {
	return Newf(errs.CodeExchange, format, args...)
}

// Network reports network issues when communicating with Binance.
func Network(format string, args ...any) *errs.E {
	return Newf(errs.CodeNetwork, format, args...)
}

// WrapExchange wraps an error with CodeExchange metadata.
func WrapExchange(err error, format string, args ...any) *errs.E {
	return Wrap(errs.CodeExchange, err, format, args...)
}

// WrapInvalid wraps an error with CodeInvalid metadata.
func WrapInvalid(err error, format string, args ...any) *errs.E {
	return Wrap(errs.CodeInvalid, err, format, args...)
}

// WrapNetwork wraps an error with CodeNetwork metadata.
func WrapNetwork(err error, format string, args ...any) *errs.E {
	return Wrap(errs.CodeNetwork, err, format, args...)
}
