package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/errs"
)

type stubProcessor struct {
	id string
}

func (p *stubProcessor) Initialize(context.Context) error { return nil }

func (p *stubProcessor) Process(context.Context, []byte) (interface{}, error) { return nil, nil }

func (p *stubProcessor) MessageTypeID() string { return p.id }

func TestMessageTypeDescriptorValidate(t *testing.T) {
	desc := MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "e",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade_processor",
	}
	if err := desc.Validate(); err != nil {
		t.Fatalf("expected nil err: %v", err)
	}

	badID := MessageTypeDescriptor{
		ID: "Trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "e",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade_processor",
	}
	if err := badID.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	emptyRules := MessageTypeDescriptor{ID: "trade", ProcessorRef: "trade_processor"}
	if err := emptyRules.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	invalidRule := MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy: DetectionStrategyFieldBased,
		}},
		ProcessorRef: "trade_processor",
	}
	if err := invalidRule.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	emptyRef := MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "e",
			ExpectedValue: "trade",
		}},
	}
	if err := emptyRef.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}
}

func TestDetectionRuleValidate(t *testing.T) {
	rule := DetectionRule{Strategy: DetectionStrategyFieldBased, FieldPath: "type", ExpectedValue: "trade", Priority: 0}
	if err := rule.Validate(); err != nil {
		t.Fatalf("expected nil err: %v", err)
	}

	negPriority := DetectionRule{Strategy: DetectionStrategyFieldBased, FieldPath: "type", ExpectedValue: "trade", Priority: -1}
	if err := negPriority.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	emptyField := DetectionRule{Strategy: DetectionStrategyFieldBased, ExpectedValue: "trade"}
	if err := emptyField.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	emptyValue := DetectionRule{Strategy: DetectionStrategyFieldBased, FieldPath: "type"}
	if err := emptyValue.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	missingSchema := DetectionRule{Strategy: DetectionStrategySchemaBased}
	if err := missingSchema.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	metadataWithField := DetectionRule{Strategy: DetectionStrategyMetadataBased, FieldPath: "type"}
	if err := metadataWithField.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}
}

func TestProcessorRegistrationValidate(t *testing.T) {
	processor := &stubProcessor{id: "trade"}
	reg := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatusAvailable, RegisteredAt: time.Now()}
	if err := reg.Validate(); err != nil {
		t.Fatalf("expected nil err: %v", err)
	}

	nilProcessor := ProcessorRegistration{MessageTypeID: "trade", Status: ProcessorStatusAvailable}
	if err := nilProcessor.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	unknownStatus := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatus(-1)}
	if err := unknownStatus.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	unavailableWithoutError := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatusUnavailable}
	if err := unavailableWithoutError.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	initializingWithError := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatusInitializing, InitError: errors.New("boom")}
	if err := initializingWithError.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	availableWithError := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatusAvailable, InitError: errors.New("boom")}
	if err := availableWithError.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}

	unavailableWithError := ProcessorRegistration{MessageTypeID: "trade", Processor: processor, Status: ProcessorStatusUnavailable, InitError: errors.New("boom")}
	if err := unavailableWithError.Validate(); err != nil {
		t.Fatalf("expected nil err: %v", err)
	}

	emptyMsgType := ProcessorRegistration{Processor: processor, Status: ProcessorStatusAvailable}
	if err := emptyMsgType.Validate(); !errors.As(err, new(*errs.E)) {
		t.Fatalf("expected errs.E, got %v", err)
	}
}
