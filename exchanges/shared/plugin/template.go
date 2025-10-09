package plugin

import (
	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
)

// Builder helps compose Registration values with inline guidance for new adapters.
type Builder struct {
	reg Registration
}

// NewBuilder constructs a Builder with the mandatory exchange name and factory.
// Typical usage occurs inside an init() function for the adapter package:
//
//	func init() {
//	    plugin.NewBuilder("my-exchange", newExchange).
//	        WithCapabilities(core.CapabilitySpotPublicREST).
//	        WithSymbolTranslator(buildSymbolTranslator).
//	        Register()
//	}
//
// The helper ensures canonical defaults (protocol version, metadata copy) while delegating
// to the adapter-specific constructor supplied via the build function.
func NewBuilder(name core.ExchangeName, build coreregistry.ExchangeFactory) *Builder {
	return &Builder{reg: Registration{Name: name, Build: build}}
}

// WithSymbolTranslator attaches a lazy symbol translator factory to the Registration.
// The factory is invoked after the exchange instance has been constructed.
func (b *Builder) WithSymbolTranslator(factory func(core.Exchange) (core.SymbolTranslator, error)) *Builder {
	b.reg.SymbolTranslator = factory
	return b
}

// WithTopicTranslator attaches a topic translator factory that will be registered post-construction.
func (b *Builder) WithTopicTranslator(factory func(core.Exchange) (core.TopicTranslator, error)) *Builder {
	b.reg.TopicTranslator = factory
	return b
}

// WithCapabilities stores the canonical capability bitset for discovery tooling.
func (b *Builder) WithCapabilities(caps ...core.Capability) *Builder {
	if len(caps) == 0 {
		return b
	}
	b.reg.Capabilities = core.Capabilities(caps...)
	return b
}

// WithProtocolVersion overrides the protocol version advertised by the adapter.
func (b *Builder) WithProtocolVersion(version string) *Builder {
	b.reg.ProtocolVersion = version
	return b
}

// WithMetadata adds a metadata key/value pair exposed through discovery summaries.
func (b *Builder) WithMetadata(key, value string) *Builder {
	if b.reg.Metadata == nil {
		b.reg.Metadata = make(map[string]string)
	}
	b.reg.Metadata[key] = value
	return b
}

// Build finalizes the Registration without side effects, applying canonical defaults.
func (b *Builder) Build() Registration {
	reg := b.reg
	if reg.ProtocolVersion == "" {
		reg.ProtocolVersion = core.ProtocolVersion
	}
	if len(reg.Metadata) != 0 {
		reg.Metadata = cloneStringMap(reg.Metadata)
	}
	return reg
}

// Register finalizes the builder and registers the adapter with shared registries.
func (b *Builder) Register() {
	Register(b.Build())
}
