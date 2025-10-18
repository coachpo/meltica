package schema

import json "github.com/goccy/go-json"

// ParseFrame captures provider-specific payloads prior to normalization.
// This is an intermediate structure used during parsing - raw exchange data
// before conversion to canonical events. Supports all exchange types.
// Complements WsFrame: WsFrame → Parser → ParseFrame → Event
type ParseFrame struct {
	returned   bool
	Provider   string
	StreamName string
	ReceivedAt int64
	Payload    json.RawMessage
}

// Reset zeroes the parse frame for pooling reuse.
func (p *ParseFrame) Reset() {
	if p == nil {
		return
	}
	p.Provider = ""
	p.StreamName = ""
	p.ReceivedAt = 0
	p.Payload = nil
	p.returned = false
}

// SetReturned updates the pooled ownership flag.
func (p *ParseFrame) SetReturned(flag bool) {
	if p == nil {
		return
	}
	p.returned = flag
}

// IsReturned reports whether the frame is currently pooled.
func (p *ParseFrame) IsReturned() bool {
	if p == nil {
		return false
	}
	return p.returned
}
