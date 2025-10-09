package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
	sharedplugin "github.com/coachpo/meltica/exchanges/shared/plugin"
)

type stubExchange struct {
	name    string
	caps    core.ExchangeCapabilities
	version string
}

func (s *stubExchange) Name() string                            { return s.name }
func (s *stubExchange) Capabilities() core.ExchangeCapabilities { return s.caps }
func (s *stubExchange) SupportedProtocolVersion() string        { return s.version }
func (s *stubExchange) Close() error                            { return nil }

func registerStub(t *testing.T, name string, caps []core.Capability, metadata map[string]string) {
	t.Helper()
	sharedplugin.Reset()
	t.Cleanup(sharedplugin.Reset)

	bitset := core.Capabilities(caps...)
	sharedplugin.Register(sharedplugin.Registration{
		Name: core.ExchangeName(name),
		Build: func(c coreregistry.Config) (core.Exchange, error) {
			return &stubExchange{name: name, caps: bitset, version: core.ProtocolVersion}, nil
		},
		Capabilities:    bitset,
		ProtocolVersion: core.ProtocolVersion,
		Metadata:        metadata,
	})
}

func TestRunTableOutput(t *testing.T) {
	caps := []core.Capability{
		core.CapabilitySpotPublicREST,
		core.CapabilitySpotTradingREST,
		core.CapabilityTradingSpotAmend,
		core.CapabilityTradingSpotCancel,
		core.CapabilityMarketTrades,
		core.CapabilityMarketTicker,
		core.CapabilityMarketOrderBook,
		core.CapabilityWebsocketPublic,
		core.CapabilityAccountBalances,
		core.CapabilityAccountPositions,
	}
	registerStub(t, "demo-exchange", caps, map[string]string{"region": "emea"})

	var out bytes.Buffer
	var errBuf bytes.Buffer
	if err := run(nil, &out, &errBuf); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected header, separator, and data row, got %d lines", len(lines))
	}
	rowFields := strings.Fields(lines[2])
	if len(rowFields) < 4 {
		t.Fatalf("expected data fields, got %v", rowFields)
	}
	if rowFields[0] != "demo-exchange" {
		t.Fatalf("unexpected exchange name %q", rowFields[0])
	}
	if rowFields[2] != "✓" {
		t.Fatalf("expected spot trading to be ✓, got %q", rowFields[2])
	}
	if rowFields[3] != "✗" {
		t.Fatalf("expected linear trading to be ✗, got %q", rowFields[3])
	}
}

func TestRunJSONOutput(t *testing.T) {
	caps := []core.Capability{
		core.CapabilitySpotPublicREST,
		core.CapabilitySpotTradingREST,
		core.CapabilityTradingSpotAmend,
		core.CapabilityTradingSpotCancel,
		core.CapabilityLinearTradingREST,
		core.CapabilityTradingLinearAmend,
		core.CapabilityTradingLinearCancel,
		core.CapabilityMarketTrades,
		core.CapabilityMarketTicker,
		core.CapabilityMarketOrderBook,
		core.CapabilityWebsocketPublic,
		core.CapabilityWebsocketPrivate,
		core.CapabilityAccountBalances,
		core.CapabilityAccountPositions,
		core.CapabilityAccountTransfers,
	}
	registerStub(t, "json-exchange", caps, map[string]string{"tier": "sandbox"})

	var out bytes.Buffer
	var errBuf bytes.Buffer
	if err := run([]string{"-json"}, &out, &errBuf); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected stderr empty, got %q", errBuf.String())
	}

	var rows []matrixRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	row := rows[0]
	if row.Name != "json-exchange" {
		t.Fatalf("unexpected name %q", row.Name)
	}
	if !row.Categories["linear_trading"] {
		t.Fatalf("expected linear trading category to be true")
	}
	if !contains(row.Capabilities, "websocket_private") {
		t.Fatalf("expected websocket_private capability, got %v", row.Capabilities)
	}
	if row.Metadata["tier"] != "sandbox" {
		t.Fatalf("unexpected metadata: %v", row.Metadata)
	}
}

func TestRunNoExchanges(t *testing.T) {
	sharedplugin.Reset()
	t.Cleanup(sharedplugin.Reset)

	var out bytes.Buffer
	var errBuf bytes.Buffer
	err := run(nil, &out, &errBuf)
	if err == nil {
		t.Fatal("expected error when no exchanges registered")
	}
	if !strings.Contains(err.Error(), "no exchanges") {
		t.Fatalf("unexpected error %v", err)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
