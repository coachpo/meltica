package router_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
	"github.com/coachpo/meltica/lib/ws-routing/router"
	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

func BenchmarkRouterDispatch(b *testing.B) {
	registry := handler.NewRegistry()
	if err := registry.Register("trade", func(ctx context.Context, event internal.Event) error {
		return nil
	}); err != nil {
		b.Fatalf("register handler: %v", err)
	}
	unit, err := router.New(router.Options{
		Registry:   registry,
		Logger:     telemetry.NewNoop(),
		BufferSize: 0,
	})
	if err != nil {
		b.Fatalf("create router: %v", err)
	}
	ctx := context.Background()
	event := internal.Event{Symbol: "BTC-USD", Type: "trade", Timestamp: time.Now()}
	if err := unit.Dispatch(ctx, event); err != nil {
		b.Fatalf("warmup dispatch failed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := unit.Dispatch(ctx, event); err != nil {
			b.Fatalf("dispatch failed: %v", err)
		}
	}
	b.StopTimer()
	const sampleSize = 100000
	durations := make([]time.Duration, sampleSize)
	for i := 0; i < sampleSize; i++ {
		start := time.Now()
		if err := unit.Dispatch(ctx, event); err != nil {
			b.Fatalf("sample dispatch failed: %v", err)
		}
		durations[i] = time.Since(start)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	median := durations[len(durations)/2]
	p99Index := int(float64(len(durations))*0.99) - 1
	if p99Index < 0 {
		p99Index = 0
	}
	if p99Index >= len(durations) {
		p99Index = len(durations) - 1
	}
	p99 := durations[p99Index]
	if median > 500*time.Nanosecond {
		b.Fatalf("median latency %s exceeds 500ns target", median)
	}
	if p99 > 2*time.Microsecond {
		b.Fatalf("p99 latency %s exceeds 2µs target", p99)
	}
}
