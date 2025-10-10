package router

import (
	"context"
	"strings"
	"sync"

	"github.com/coachpo/meltica/errs"
)

// RouterDispatcher coordinates delivery of raw messages to processor inboxes with backpressure.
type RouterDispatcher struct {
	ctx         context.Context
	cancel      context.CancelFunc
	workers     sync.Map // messageTypeID -> *processorWorker
	defaultWork *processorWorker
	flow        *FlowController
}

// NewRouterDispatcher creates a dispatcher bound to the provided context.
func NewRouterDispatcher(ctx context.Context, metrics *RoutingMetrics) *RouterDispatcher {
	child, cancel := context.WithCancel(ctx)
	return &RouterDispatcher{ctx: child, cancel: cancel, flow: NewFlowController(metrics)}
}

// Bind links a message type registration to a newly created inbox channel.
func (d *RouterDispatcher) Bind(messageTypeID string, reg *ProcessorRegistration) <-chan []byte {
	worker := newProcessorWorker(messageTypeID, reg)
	d.workers.Store(messageTypeID, worker)
	return worker.inbox
}

// BindDefault assigns the fallback worker used when message type lookups miss.
func (d *RouterDispatcher) BindDefault(reg *ProcessorRegistration) <-chan []byte {
	messageTypeID := reg.MessageTypeID
	if strings.TrimSpace(messageTypeID) == "" {
		messageTypeID = "default"
	}
	worker := newProcessorWorker(messageTypeID, reg)
	d.defaultWork = worker
	return worker.inbox
}

// Dispatch delivers raw bytes to the target processor, blocking until consumed or context cancellation occurs.
func (d *RouterDispatcher) Dispatch(messageTypeID string, raw []byte) error {
	if d == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("dispatcher not initialized"))
	}
	worker := d.lookupWorker(messageTypeID)
	if worker == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor channel not registered"))
	}
	if d.flow == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("dispatcher flow controller unavailable"))
	}
	return d.flow.Send(d.ctx, worker.messageTypeID, worker.inbox, raw)
}

// Shutdown stops dispatch operations and releases resources.
func (d *RouterDispatcher) Shutdown() {
	if d == nil {
		return
	}
	d.cancel()
}

func (d *RouterDispatcher) lookupWorker(messageTypeID string) *processorWorker {
	if messageTypeID != "" {
		if value, ok := d.workers.Load(messageTypeID); ok {
			if worker, cast := value.(*processorWorker); cast {
				return worker
			}
		}
	}
	return d.defaultWork
}

type processorWorker struct {
	registration  *ProcessorRegistration
	inbox         chan []byte
	messageTypeID string
}

func newProcessorWorker(messageTypeID string, reg *ProcessorRegistration) *processorWorker {
	return &processorWorker{
		registration:  reg,
		inbox:         make(chan []byte),
		messageTypeID: messageTypeID,
	}
}
