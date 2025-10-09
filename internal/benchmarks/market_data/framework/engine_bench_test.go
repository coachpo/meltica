package frameworkbench

import (
	"runtime"
	"testing"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/connection"
	"github.com/coachpo/meltica/market_data/framework/parser"
)

var benchmarkPayload = []byte(`{"type":"quote","symbol":"BTC-USDT","bid":"43123.12","ask":"43123.55","ts":1697049600000}`)

func BenchmarkEnginePipelineFanout(b *testing.B) {
	pool, err := connection.NewPoolHandle(connection.PoolOptions{MaxMessageBytes: 1 << 16})
	if err != nil {
		b.Fatalf("pool init failed: %v", err)
	}
	pipeline := parser.NewJSONPipeline(
		pool.AcquireEnvelope,
		pool.ReleaseEnvelope,
		func(payload []byte) (parser.DecoderLease, error) {
			return pool.BorrowDecoder(payload)
		},
		pool.MaxMessageBytes(),
	)
	stage := parser.NewValidationStage(
		parser.WithValidators(parser.ValidatorFunc(quoteValidator)),
	)
	consumers := make([]benchConsumer, 8)

	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkPayload)))

	// Warm-up to populate pools.
	env, err := pipeline.Decode(benchmarkPayload)
	if err != nil {
		b.Fatalf("warmup decode failed: %v", err)
	}
	if result := stage.Validate(env); !result.Valid {
		b.Fatalf("warmup validation failed: %v", result.Err)
	}
	pool.ReleaseEnvelope(env)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope, err := pipeline.Decode(benchmarkPayload)
		if err != nil {
			b.Fatalf("decode failed: %v", err)
		}
		result := stage.Validate(envelope)
		if !result.Valid {
			b.Fatalf("validation failed: %v", result.Err)
		}
		for idx := range consumers {
			consumers[idx].Consume(envelope)
		}
		pool.ReleaseEnvelope(envelope)
	}
	b.StopTimer()
	runtime.KeepAlive(consumers)
}

type benchConsumer struct {
	processed uint64
}

func (c *benchConsumer) Consume(env *framework.MessageEnvelope) {
	if env == nil {
		return
	}
	c.processed++
}

func quoteValidator(env *framework.MessageEnvelope) error {
	payload, ok := env.Decoded().(map[string]any)
	if !ok {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("unexpected payload type"))
	}
	symbol, _ := payload["symbol"].(string)
	if symbol == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("symbol required"))
	}
	bid := payload["bid"]
	ask := payload["ask"]
	if bid == nil || ask == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("quote missing prices"))
	}
	env.SetValidated(true)
	return nil
}

func init() {
	// Ensure reference structs retain reasonable defaults for throughput reporting.
	time.Local = time.UTC
}
