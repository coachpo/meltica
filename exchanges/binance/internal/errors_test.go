package internal

import (
	"errors"
	"strings"
	"testing"

	"github.com/coachpo/meltica/errs"
)

func TestNewAndNewf(t *testing.T) {
	basic := New(errs.CodeInvalid, " invalid payload ")
	if basic.Exchange != exchange {
		t.Fatalf("expected exchange %q, got %q", exchange, basic.Exchange)
	}
	if basic.Code != errs.CodeInvalid {
		t.Fatalf("expected code %s, got %s", errs.CodeInvalid, basic.Code)
	}
	if basic.Message != "invalid payload" {
		t.Fatalf("unexpected message: %q", basic.Message)
	}

	formatted := Newf(errs.CodeNetwork, "dial %s failed", "ws")
	if !strings.Contains(formatted.Message, "dial ws failed") {
		t.Fatalf("expected formatted message, got %q", formatted.Message)
	}
}

func TestWrapBehavior(t *testing.T) {
	cause := errors.New("boom")
	wrapped := Wrap(errs.CodeExchange, cause, "operation %s", "fail")
	if wrapped.Code != errs.CodeExchange {
		t.Fatalf("expected exchange code, got %s", wrapped.Code)
	}
	if wrapped.Unwrap() != cause {
		t.Fatalf("expected cause propagation")
	}
	if !strings.Contains(wrapped.Message, "operation fail: boom") {
		t.Fatalf("expected message to include cause, got %q", wrapped.Message)
	}

	withoutCause := Wrap(errs.CodeExchange, nil, "noop")
	if withoutCause.Unwrap() != nil {
		t.Fatalf("expected nil cause")
	}
	if withoutCause.Message != "noop" {
		t.Fatalf("expected message passthrough, got %q", withoutCause.Message)
	}
}

func TestHelperConstructors(t *testing.T) {
	invalid := Invalid("bad %s", "symbol")
	if invalid.Code != errs.CodeInvalid {
		t.Fatalf("expected invalid code, got %s", invalid.Code)
	}

	exchangeErr := Exchange("downstream")
	if exchangeErr.Code != errs.CodeExchange {
		t.Fatalf("expected exchange code, got %s", exchangeErr.Code)
	}

	network := Network("timeout")
	if network.Code != errs.CodeNetwork {
		t.Fatalf("expected network code, got %s", network.Code)
	}

	wrappedNet := WrapNetwork(errors.New("oops"), "call")
	if wrappedNet.Code != errs.CodeNetwork {
		t.Fatalf("expected network code, got %s", wrappedNet.Code)
	}
	if wrappedNet.Unwrap() == nil {
		t.Fatalf("expected underlying cause to be preserved")
	}

	wrappedInvalid := WrapInvalid(nil, "validation")
	if wrappedInvalid.Code != errs.CodeInvalid {
		t.Fatalf("expected invalid code, got %s", wrappedInvalid.Code)
	}
	if wrappedInvalid.Unwrap() != nil {
		t.Fatalf("expected nil cause when none provided")
	}

	wrappedExchange := WrapExchange(errors.New("down"), "bridge")
	if wrappedExchange.Code != errs.CodeExchange {
		t.Fatalf("expected exchange code, got %s", wrappedExchange.Code)
	}
}
