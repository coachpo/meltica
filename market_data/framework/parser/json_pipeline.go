package parser

import (
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
)

// DecoderLease exposes the subset of decoder behaviors required by the pipeline.
//
// Deprecated: Implement processors.Processor and rely on the routing framework
// to handle decoding rather than borrowing decoder leases directly.
type DecoderLease interface {
	Decode(dst any, opts ...json.DecodeOptionFunc) error
	Release()
}

// JSONPipeline manages pooled decoding of JSON payloads into message envelopes.
//
// Deprecated: Replace usages with router-driven processor execution. Processors
// receive raw payload bytes and are responsible for decoding into typed models.
type JSONPipeline struct {
	acquire func() *framework.MessageEnvelope
	release func(*framework.MessageEnvelope)
	borrow  func([]byte) (DecoderLease, error)
	maxSize int
}

// NewJSONPipeline constructs a decoder pipeline backed by pooled resources.
//
// Deprecated: Instantiate processors and register them with the routing table
// instead of constructing JSONPipeline instances.
func NewJSONPipeline(
	acquire func() *framework.MessageEnvelope,
	release func(*framework.MessageEnvelope),
	borrow func([]byte) (DecoderLease, error),
	maxSize int,
) *JSONPipeline {
	return &JSONPipeline{acquire: acquire, release: release, borrow: borrow, maxSize: maxSize}
}

// Decode transforms a JSON payload into a pooled MessageEnvelope.
//
// When opts are provided, they augment the decoder configuration beyond the
// defaults set on the shared pool.
//
// Deprecated: Processors should decode payloads directly and return typed
// models to the router rather than producing MessageEnvelopes through the
// parser pipeline.
func (p *JSONPipeline) Decode(payload []byte, opts ...json.DecodeOptionFunc) (*framework.MessageEnvelope, error) {
	if p == nil || p.acquire == nil || p.release == nil || p.borrow == nil || p.maxSize <= 0 {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("json pipeline not configured"))
	}
	if len(payload) > p.maxSize {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("payload exceeds configured limit"))
	}
	env := p.acquire()
	buffer := env.Raw()
	buffer = append(buffer[:0], payload...)
	env.SetRaw(buffer)
	env.SetReceivedAt(time.Now().UTC())
	lease, err := p.borrow(buffer)
	if err != nil {
		p.release(env)
		return nil, err
	}
	defer lease.Release()
	var decoded map[string]any
	if err := lease.Decode(&decoded, opts...); err != nil {
		p.release(env)
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("json decode failed"), errs.WithCause(err))
	}
	env.SetDecoded(decoded)
	return env, nil
}
