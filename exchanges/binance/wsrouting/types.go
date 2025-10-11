package wsrouting

import (
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/processors"
)

// MessageTypeDescriptor describes how to detect and route a specific message type.
type MessageTypeDescriptor struct {
	ID             string
	DisplayName    string
	DetectionRules []DetectionRule
	ProcessorRef   string
	SchemaVersion  string
	CreatedAt      time.Time
}

// Validate ensures the descriptor satisfies core routing invariants.
func (d MessageTypeDescriptor) Validate() error {
	trimmedID := strings.TrimSpace(d.ID)
	if trimmedID == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("message type descriptor id required"))
	}
	if strings.ToLower(trimmedID) != trimmedID {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("message type descriptor id must be lowercase"))
	}
	if len(d.DetectionRules) == 0 {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("at least one detection rule required"))
	}
	for idx, rule := range d.DetectionRules {
		if err := rule.Validate(); err != nil {
			msg := fmt.Sprintf("detection rule %d invalid", idx)
			return errs.New("", errs.CodeInvalid, errs.WithMessage(msg), errs.WithCause(err))
		}
	}
	if strings.TrimSpace(d.ProcessorRef) == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor ref required"))
	}
	return nil
}

// DetectionStrategy describes how a rule determines a message type.
type DetectionStrategy int

const (
	// DetectionStrategyFieldBased inspects a JSON field value.
	DetectionStrategyFieldBased DetectionStrategy = iota
	// DetectionStrategySchemaBased validates against a JSON schema (reserved for future use).
	DetectionStrategySchemaBased
	// DetectionStrategyMetadataBased relies on out-of-band stream metadata.
	DetectionStrategyMetadataBased
)

// String returns the textual form of the strategy.
func (s DetectionStrategy) String() string {
	switch s {
	case DetectionStrategyFieldBased:
		return "field"
	case DetectionStrategySchemaBased:
		return "schema"
	case DetectionStrategyMetadataBased:
		return "metadata"
	default:
		return "unknown"
	}
}

// DetectionRule defines a single detection step evaluated during routing.
type DetectionRule struct {
	Strategy      DetectionStrategy
	FieldPath     string
	ExpectedValue string
	SchemaURI     string
	Priority      int
}

// Validate ensures the detection rule is internally consistent.
func (r DetectionRule) Validate() error {
	if !r.Strategy.valid() {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("unsupported detection strategy"))
	}
	if r.Priority < 0 {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("detection rule priority must be non-negative"))
	}
	switch r.Strategy {
	case DetectionStrategyFieldBased:
		if strings.TrimSpace(r.FieldPath) == "" {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("field path required for field strategy"))
		}
		if strings.TrimSpace(r.ExpectedValue) == "" {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("expected value required for field strategy"))
		}
	case DetectionStrategySchemaBased:
		if strings.TrimSpace(r.SchemaURI) == "" {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("schema uri required for schema strategy"))
		}
	case DetectionStrategyMetadataBased:
		if strings.TrimSpace(r.FieldPath) != "" || strings.TrimSpace(r.ExpectedValue) != "" {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("metadata strategy must not declare field path or expected value"))
		}
	}
	return nil
}

func (s DetectionStrategy) valid() bool {
	switch s {
	case DetectionStrategyFieldBased, DetectionStrategySchemaBased, DetectionStrategyMetadataBased:
		return true
	default:
		return false
	}
}

// ProcessorStatus expresses the current lifecycle state of a processor registration.
type ProcessorStatus int

const (
	// ProcessorStatusInitializing indicates a processor is starting up.
	ProcessorStatusInitializing ProcessorStatus = iota
	// ProcessorStatusAvailable indicates the processor is ready for use.
	ProcessorStatusAvailable
	// ProcessorStatusUnavailable indicates initialization failed and the processor is disabled.
	ProcessorStatusUnavailable
)

// String returns the machine-friendly representation of the status.
func (s ProcessorStatus) String() string {
	switch s {
	case ProcessorStatusInitializing:
		return "initializing"
	case ProcessorStatusAvailable:
		return "available"
	case ProcessorStatusUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

func (s ProcessorStatus) valid() bool {
	switch s {
	case ProcessorStatusInitializing, ProcessorStatusAvailable, ProcessorStatusUnavailable:
		return true
	default:
		return false
	}
}

// ProcessorRegistration associates a message type with its processor implementation and operational state.
type ProcessorRegistration struct {
	MessageTypeID string
	Processor     processors.Processor
	Status        ProcessorStatus
	InitError     error
	RegisteredAt  time.Time
	InitDuration  time.Duration
}

// Validate ensures registration invariants and lifecycle rules hold.
func (r ProcessorRegistration) Validate() error {
	if strings.TrimSpace(r.MessageTypeID) == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor registration message type id required"))
	}
	if r.Processor == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor instance required"))
	}
	if !r.Status.valid() {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("unknown processor status"))
	}
	switch r.Status {
	case ProcessorStatusInitializing:
		if r.InitError != nil {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("initializing processor must not report init error"))
		}
	case ProcessorStatusAvailable:
		if r.InitError != nil {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("available processor must not carry init error"))
		}
	case ProcessorStatusUnavailable:
		if r.InitError == nil {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("unavailable processor requires init error"))
		}
	}
	return nil
}
