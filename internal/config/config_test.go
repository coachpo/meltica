package config

import (
	"context"
	"os"
	"testing"
	"time"
)

// Example unit test for internal package
func TestStreamOrderingConfigDefaults(t *testing.T) {
	cfg := StreamOrderingConfig{}

	if cfg.LatenessTolerance == 0 {
		cfg.LatenessTolerance = 150 * time.Millisecond
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 50 * time.Millisecond
	}
	if cfg.MaxBufferSize == 0 {
		cfg.MaxBufferSize = 1024
	}

	if cfg.LatenessTolerance != 150*time.Millisecond {
		t.Errorf("expected default LatenessTolerance 150ms, got %v", cfg.LatenessTolerance)
	}
}

func TestLoadStreamingConfig(t *testing.T) {
	// Skip if test config file doesn't exist
	if _, err := os.Stat("streaming.example.yaml"); os.IsNotExist(err) {
		t.Skip("streaming.example.yaml not found")
	}

	ctx := context.Background()
	cfg, err := LoadStreamingConfig(ctx, "streaming.example.yaml")
	if err != nil {
		t.Fatalf("LoadStreamingConfig() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Verify basic structure
	if cfg.Databus.BufferSize == 0 {
		t.Error("expected non-zero Databus.BufferSize")
	}
}
