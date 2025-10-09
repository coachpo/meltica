package bootstrap

import (
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
)

// ValidateProtocol enforces the protocol handshake between an exchange adapter and the core runtime.
// CQ-08 requires adapters to keep interfaces immutable across protocol revisions, so any version
// drift is treated as a construction error.
func ValidateProtocol(exchange core.Exchange) error {
	if exchange == nil {
		return fmt.Errorf("bootstrap: exchange is nil")
	}
	version := strings.TrimSpace(exchange.SupportedProtocolVersion())
	if version == "" {
		return fmt.Errorf("bootstrap: exchange %s reported empty protocol version", exchange.Name())
	}
	if version != core.ProtocolVersion {
		return fmt.Errorf(
			"bootstrap: exchange %s implements protocol %s, expected %s",
			exchange.Name(), version, core.ProtocolVersion,
		)
	}
	return nil
}

// RequireProtocol panics when ValidateProtocol fails, providing a fail-fast guard for init paths.
func RequireProtocol(exchange core.Exchange) {
	if err := ValidateProtocol(exchange); err != nil {
		panic(err)
	}
}
