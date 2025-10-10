package router

import (
	"errors"
	"testing"

	"github.com/coachpo/meltica/errs"
)

func TestFieldDetectionStrategyDetect(t *testing.T) {
	rule := DetectionRule{
		Strategy:      DetectionStrategyFieldBased,
		FieldPath:     "type",
		ExpectedValue: "trade",
	}
	strategy, err := NewFieldDetectionStrategy(rule, "trade")
	if err != nil {
		t.Fatalf("strategy init: %v", err)
	}

	payload := loadTestPayload(t, "trade.json")
	msgType, err := strategy.Detect(payload)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if msgType != "trade" {
		t.Fatalf("expected trade, got %s", msgType)
	}
}

func TestFieldDetectionStrategyDetectMismatch(t *testing.T) {
	rule := DetectionRule{
		Strategy:      DetectionStrategyFieldBased,
		FieldPath:     "type",
		ExpectedValue: "funding",
	}
	strategy, err := NewFieldDetectionStrategy(rule, "funding")
	if err != nil {
		t.Fatalf("strategy init: %v", err)
	}

	payload := loadTestPayload(t, "trade.json")
	_, detectErr := strategy.Detect(payload)
	if detectErr == nil {
		t.Fatalf("expected mismatch error")
	}
	var e *errs.E
	if !errors.As(detectErr, &e) {
		t.Fatalf("expected errs.E, got %T", detectErr)
	}
}

func TestFieldDetectionStrategyDetectMissingField(t *testing.T) {
	rule := DetectionRule{
		Strategy:      DetectionStrategyFieldBased,
		FieldPath:     "metadata.type",
		ExpectedValue: "trade",
	}
	strategy, err := NewFieldDetectionStrategy(rule, "trade")
	if err != nil {
		t.Fatalf("strategy init: %v", err)
	}

	payload := loadTestPayload(t, "trade.json")
	_, detectErr := strategy.Detect(payload)
	if detectErr == nil {
		t.Fatalf("expected missing field error")
	}
}

func TestFieldDetectionStrategyDetectInvalidJSON(t *testing.T) {
	rule := DetectionRule{
		Strategy:      DetectionStrategyFieldBased,
		FieldPath:     "type",
		ExpectedValue: "trade",
	}
	strategy, err := NewFieldDetectionStrategy(rule, "trade")
	if err != nil {
		t.Fatalf("strategy init: %v", err)
	}

	_, detectErr := strategy.Detect([]byte("{invalid"))
	if detectErr == nil {
		t.Fatalf("expected parse error")
	}
}
