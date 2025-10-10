package bootstrap

import (
	"testing"

	"github.com/coachpo/meltica/core"
)

type protocolStub struct {
	name    string
	caps    core.ExchangeCapabilities
	version string
}

func (p protocolStub) Name() string                            { return p.name }
func (p protocolStub) Capabilities() core.ExchangeCapabilities { return p.caps }
func (p protocolStub) SupportedProtocolVersion() string        { return p.version }
func (protocolStub) Close() error                              { return nil }

func TestValidateProtocolMatch(t *testing.T) {
	stub := protocolStub{name: "match", version: core.ProtocolVersion}
	if err := ValidateProtocol(stub); err != nil {
		t.Fatalf("expected match to succeed, got %v", err)
	}
}

func TestValidateProtocolMismatch(t *testing.T) {
	stub := protocolStub{name: "mismatch", version: "0.9.0"}
	if err := ValidateProtocol(stub); err == nil {
		t.Fatal("expected mismatch to error")
	}
}

func TestValidateProtocolEmptyVersion(t *testing.T) {
	stub := protocolStub{name: "empty", version: "   "}
	if err := ValidateProtocol(stub); err == nil {
		t.Fatal("expected empty version to error")
	}
}

func TestRequireProtocolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for protocol mismatch")
		}
	}()
	RequireProtocol(protocolStub{name: "panic", version: "1.2.0"})
}
