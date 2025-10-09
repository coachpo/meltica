package connection

import (
	"bytes"
	"fmt"
	"sync"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
)

const defaultMaxMessageBytes = 1 << 16

// PoolOptions configures pooled resource behavior.
type PoolOptions struct {
	MaxMessageBytes int
	DecoderOptions  []json.DecodeOptionFunc
}

// PoolHandle provides pooled envelopes, decoders, and buffers for the engine hot path.
type PoolHandle struct {
	envelopePool    sync.Pool
	decoderPool     sync.Pool
	bufferPool      sync.Pool
	maxMessageBytes int
	decoderOpts     []json.DecodeOptionFunc
}

type decoderState struct {
	reader  *bytes.Reader
	decoder *json.Decoder
}

// DecoderLease owns a borrowed decoder until released.
type DecoderLease struct {
	pool  *PoolHandle
	state *decoderState
}

// NewPoolHandle constructs a pooled resource manager.
func NewPoolHandle(opts PoolOptions) (*PoolHandle, error) {
	if opts.MaxMessageBytes < 0 {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("max message bytes must be non-negative"))
	}
	if opts.MaxMessageBytes == 0 {
		opts.MaxMessageBytes = defaultMaxMessageBytes
	}
	if opts.MaxMessageBytes == 0 {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("max message bytes must be greater than zero"))
	}
	handle := &PoolHandle{
		maxMessageBytes: opts.MaxMessageBytes,
		decoderOpts:     append([]json.DecodeOptionFunc(nil), opts.DecoderOptions...),
	}
	handle.envelopePool.New = func() any {
		return &framework.MessageEnvelope{
			RawData:    make([]byte, 0, handle.maxMessageBytes),
			ErrorsList: make([]error, 0, 4),
		}
	}
	handle.decoderPool.New = func() any {
		reader := bytes.NewReader(nil)
		decoder := json.NewDecoder(reader)
		decoder.DisallowUnknownFields()
		decoder.UseNumber()
		return &decoderState{reader: reader, decoder: decoder}
	}
	handle.bufferPool.New = func() any {
		return make([]byte, 0, handle.maxMessageBytes)
	}
	return handle, nil
}

// AcquireEnvelope borrows a message envelope with an attached buffer.
func (p *PoolHandle) AcquireEnvelope() *framework.MessageEnvelope {
	env := p.envelopePool.Get().(*framework.MessageEnvelope)
	env.Reset()
	buf := p.BorrowBuffer()
	env.RawData = buf[:0]
	return env
}

// ReleaseEnvelope returns an envelope and its buffer to the pool.
func (p *PoolHandle) ReleaseEnvelope(env *framework.MessageEnvelope) {
	if env == nil {
		return
	}
	if env.RawData != nil {
		p.ReleaseBuffer(env.RawData)
		env.RawData = nil
	}
	env.Reset()
	p.envelopePool.Put(env)
}

// BorrowBuffer obtains a scratch buffer sized to the configured message limit.
func (p *PoolHandle) BorrowBuffer() []byte {
	buf := p.bufferPool.Get().([]byte)
	if cap(buf) < p.maxMessageBytes {
		return make([]byte, 0, p.maxMessageBytes)
	}
	return buf[:0]
}

// ReleaseBuffer returns a buffer to the pool when capacity matches expectations.
func (p *PoolHandle) ReleaseBuffer(buf []byte) {
	if buf == nil {
		return
	}
	if cap(buf) != p.maxMessageBytes {
		return
	}
	p.bufferPool.Put(buf[:0])
}

// BorrowDecoder provides a decoder lease tied to the given payload.
func (p *PoolHandle) BorrowDecoder(payload []byte) (*DecoderLease, error) {
	if len(payload) > p.maxMessageBytes {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage(fmt.Sprintf("payload exceeds max size: %d > %d", len(payload), p.maxMessageBytes)))
	}
	state := p.decoderPool.Get().(*decoderState)
	state.reader.Reset(payload)
	return &DecoderLease{pool: p, state: state}, nil
}

// Decoder exposes the underlying json.Decoder.
func (l *DecoderLease) Decoder() *json.Decoder {
	if l == nil || l.state == nil {
		return nil
	}
	return l.state.decoder
}

// Decode applies configured options and custom overrides before decoding into dst.
func (l *DecoderLease) Decode(dst any, extraOpts ...json.DecodeOptionFunc) error {
	if l == nil || l.state == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("decoder lease released"))
	}
	if len(l.pool.decoderOpts) == 0 && len(extraOpts) == 0 {
		return l.state.decoder.Decode(dst)
	}
	opts := make([]json.DecodeOptionFunc, 0, len(l.pool.decoderOpts)+len(extraOpts))
	opts = append(opts, l.pool.decoderOpts...)
	opts = append(opts, extraOpts...)
	return l.state.decoder.DecodeWithOption(dst, opts...)
}

// Release returns the decoder to the pool.
func (l *DecoderLease) Release() {
	if l == nil || l.state == nil {
		return
	}
	l.state.reader.Reset(nil)
	l.pool.decoderPool.Put(l.state)
	l.state = nil
	l.pool = nil
}

// MaxMessageBytes reports the configured message size ceiling.
func (p *PoolHandle) MaxMessageBytes() int {
	return p.maxMessageBytes
}
