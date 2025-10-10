package processors

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
)

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor context cancelled"), errs.WithCause(err))
	}
	return nil
}

func parseDecimal(field, value string, requirePositive bool) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, invalidFieldError(field, "value required", nil)
	}
	rat := new(big.Rat)
	if _, ok := rat.SetString(trimmed); !ok {
		return nil, invalidFieldError(field, "invalid decimal", fmt.Errorf("invalid rational %q", value))
	}
	if requirePositive && rat.Sign() <= 0 {
		return nil, invalidFieldError(field, "must be positive", nil)
	}
	return rat, nil
}

func parseSide(value string) (core.OrderSide, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "buy":
		return core.SideBuy, nil
	case "sell":
		return core.SideSell, nil
	default:
		return "", invalidFieldError("side", "unsupported value", fmt.Errorf("value %q", value))
	}
}

func parseTimestamp(field, value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, invalidFieldError(field, "value required", nil)
	}
	ts, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, invalidFieldError(field, "invalid timestamp", err)
	}
	return ts, nil
}

func invalidFieldError(field, message string, cause error) error {
	msg := fmt.Sprintf("%s %s", field, message)
	opts := []errs.Option{errs.WithMessage(msg), errs.WithVenueField("field", field)}
	if cause != nil {
		opts = append(opts, errs.WithCause(cause))
	}
	return errs.New("", errs.CodeInvalid, opts...)
}

func decodeError(message string, cause error) error {
	return errs.New("", errs.CodeInvalid, errs.WithMessage(message), errs.WithCause(cause))
}
