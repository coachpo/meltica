package wsrouting

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/processors"
)

// RoutingTable maintains processor registrations and descriptors for routing decisions.
type RoutingTable struct {
	entries         sync.Map // messageTypeID -> *ProcessorRegistration
	descriptors     map[string]*MessageTypeDescriptor
	strategies      map[string][]detectionStrategy
	descriptorOrder []string
	descMu          sync.RWMutex
	defaultProc     *ProcessorRegistration
	metrics         *RoutingMetrics
}

type detectionStrategy interface {
	Detect(raw []byte) (string, error)
}

// NewRoutingTable constructs an empty routing table.
func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		descriptors: make(map[string]*MessageTypeDescriptor),
		strategies:  make(map[string][]detectionStrategy),
		metrics:     NewRoutingMetrics(),
	}
}

// Register stores the descriptor and processor, invoking initialization with a five-second timeout.
func (rt *RoutingTable) Register(desc *MessageTypeDescriptor, proc processors.Processor) error {
	if desc == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("descriptor required"))
	}
	if proc == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor instance required"))
	}
	if err := desc.Validate(); err != nil {
		return err
	}
	if id := strings.TrimSpace(proc.MessageTypeID()); id == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor message type id required"))
	} else if id != desc.ID {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor message type mismatch"))
	}

	compiledDesc, compiledStrategies, err := rt.compileDescriptor(desc)
	if err != nil {
		return err
	}

	registration := &ProcessorRegistration{
		MessageTypeID: desc.ID,
		Processor:     proc,
		Status:        ProcessorStatusInitializing,
		RegisteredAt:  time.Now(),
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := proc.Initialize(ctx); err != nil {
		registration.Status = ProcessorStatusUnavailable
		registration.InitError = errs.New("", errs.CodeInvalid, errs.WithMessage("processor initialization failed"), errs.WithCause(err))
		if rt.metrics != nil {
			rt.metrics.mu.Lock()
			rt.metrics.ProcessorInitFailures++
			rt.metrics.mu.Unlock()
		}
	} else {
		registration.Status = ProcessorStatusAvailable
	}
	registration.InitDuration = time.Since(start)

	if err := registration.Validate(); err != nil {
		return err
	}

	rt.entries.Store(desc.ID, registration)
	rt.descMu.Lock()
	if _, exists := rt.descriptors[desc.ID]; !exists {
		rt.descriptorOrder = append(rt.descriptorOrder, desc.ID)
	}
	rt.descriptors[desc.ID] = compiledDesc
	rt.strategies[desc.ID] = compiledStrategies
	rt.descMu.Unlock()
	return nil
}

// Lookup returns the processor registration for the supplied message type ID.
// When no registration exists, the default processor is returned.
func (rt *RoutingTable) Lookup(messageTypeID string) *ProcessorRegistration {
	trimmed := strings.TrimSpace(messageTypeID)
	if trimmed != "" {
		if value, ok := rt.entries.Load(trimmed); ok {
			if reg, cast := value.(*ProcessorRegistration); cast {
				return reg
			}
		}
	}
	return rt.defaultProc
}

// SetDefault configures the fallback processor for unrecognized message types.
func (rt *RoutingTable) SetDefault(proc processors.Processor) error {
	if proc == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("default processor required"))
	}

	registration := &ProcessorRegistration{
		MessageTypeID: strings.TrimSpace(proc.MessageTypeID()),
		Processor:     proc,
		Status:        ProcessorStatusInitializing,
		RegisteredAt:  time.Now(),
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := proc.Initialize(ctx); err != nil {
		registration.Status = ProcessorStatusUnavailable
		registration.InitError = errs.New("", errs.CodeInvalid, errs.WithMessage("default processor initialization failed"), errs.WithCause(err))
		if rt.metrics != nil {
			rt.metrics.mu.Lock()
			rt.metrics.ProcessorInitFailures++
			rt.metrics.mu.Unlock()
		}
	} else {
		registration.Status = ProcessorStatusAvailable
	}

	registration.InitDuration = time.Since(start)

	if err := registration.Validate(); err != nil {
		return err
	}

	rt.defaultProc = registration
	return nil
}

func (rt *RoutingTable) compileDescriptor(desc *MessageTypeDescriptor) (*MessageTypeDescriptor, []detectionStrategy, error) {
	clone := *desc
	if len(desc.DetectionRules) > 0 {
		clone.DetectionRules = make([]DetectionRule, len(desc.DetectionRules))
		copy(clone.DetectionRules, desc.DetectionRules)
	}
	strategies, err := buildDetectionStrategies(&clone)
	if err != nil {
		return nil, nil, err
	}
	return &clone, strategies, nil
}

// Detect evaluates registered descriptors in priority order and returns the first matching message type.
func (rt *RoutingTable) Detect(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", newDetectionError("raw payload required for detection", nil, false)
	}

	rt.descMu.RLock()
	order := make([]string, len(rt.descriptorOrder))
	copy(order, rt.descriptorOrder)
	strategies := make(map[string][]detectionStrategy, len(rt.strategies))
	for id, compiled := range rt.strategies {
		strategies[id] = compiled
	}
	rt.descMu.RUnlock()

	var lastErr error
	for _, id := range order {
		rules := strategies[id]
		for _, strat := range rules {
			messageTypeID, err := strat.Detect(raw)
			if err != nil {
				if isNoMatchError(err) {
					lastErr = err
					continue
				}
				if rt.metrics != nil {
					rt.metrics.RecordError()
				}
				return "", err
			}
			if rt.metrics != nil {
				rt.metrics.RecordRoute(messageTypeID)
			}
			return messageTypeID, nil
		}
	}
	if lastErr != nil {
		if rt.metrics != nil {
			rt.metrics.RecordError()
		}
		return "", lastErr
	}
	if rt.metrics != nil {
		rt.metrics.RecordError()
	}
	return "", newDetectionError("no detection rule matched", nil, true)
}

func buildDetectionStrategies(desc *MessageTypeDescriptor) ([]detectionStrategy, error) {
	rules := make([]DetectionRule, len(desc.DetectionRules))
	copy(rules, desc.DetectionRules)
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return i < j
		}
		return rules[i].Priority < rules[j].Priority
	})

	strategies := make([]detectionStrategy, 0, len(rules))
	for _, rule := range rules {
		switch rule.Strategy {
		case DetectionStrategyFieldBased:
			strat, err := NewFieldDetectionStrategy(rule, desc.ID)
			if err != nil {
				return nil, err
			}
			strategies = append(strategies, strat)
		default:
			return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("unsupported detection strategy"))
		}
	}
	// Preserve sorted order on descriptor clone to aid inspection.
	desc.DetectionRules = rules
	return strategies, nil
}

// GetMetrics returns a snapshot of routing metrics.
func (rt *RoutingTable) GetMetrics() *RoutingMetrics {
	if rt == nil || rt.metrics == nil {
		return nil
	}
	return rt.metrics.Snapshot()
}
