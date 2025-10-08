package rest

import (
	"errors"
	"testing"

	"github.com/coachpo/meltica/errs"
)

func TestMapHTTPErrorCanonicalOrderNotFound(t *testing.T) {
	err := mapHTTPError(400, []byte(`{"code":-2013,"msg":"Order does not exist."}`))
	assertCanonical(t, err, errs.CanonicalOrderNotFound)
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected errs.E, got %T", err)
	}
	if e.RawCode != "-2013" {
		t.Fatalf("expected raw code -2013, got %s", e.RawCode)
	}
}

func TestMapHTTPErrorCanonicalInsufficientBalance(t *testing.T) {
	err := mapHTTPError(400, []byte(`{"code":-2010,"msg":"Account has insufficient balance for requested action."}`))
	assertCanonical(t, err, errs.CanonicalInsufficientBalance)
}

func TestMapHTTPErrorCanonicalInvalidSymbol(t *testing.T) {
	err := mapHTTPError(400, []byte(`{"code":-1121,"msg":"Invalid symbol."}`))
	assertCanonical(t, err, errs.CanonicalInvalidSymbol)
}

func TestMapHTTPErrorCanonicalRateLimited(t *testing.T) {
	err := mapHTTPError(429, []byte(`{"code":-1003,"msg":"Too many requests"}`))
	assertCanonical(t, err, errs.CanonicalRateLimited)
}

func assertCanonical(t *testing.T, err error, expected errs.CanonicalCode) {
	t.Helper()
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected errs.E, got %T", err)
	}
	if e.Canonical != expected {
		t.Fatalf("expected canonical %s, got %s", expected, e.Canonical)
	}
}
