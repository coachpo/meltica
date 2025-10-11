package routing

import (
	"context"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/errs"
	bnwsrouting "github.com/coachpo/meltica/exchanges/binance/wsrouting"
)

type processedResult struct {
	msg *corestreams.RoutedMessage
	err error
}

type processorHub struct {
	ctx        context.Context
	cancel     context.CancelFunc
	dispatcher *bnwsrouting.RouterDispatcher
	outputs    map[string]chan processedResult
}

func newProcessorHub(parent context.Context, table *bnwsrouting.RoutingTable, dispatcher *bnwsrouting.RouterDispatcher, descriptors []*bnwsrouting.MessageTypeDescriptor) *processorHub {
	ctx, cancel := context.WithCancel(parent)
	hub := &processorHub{
		ctx:        ctx,
		cancel:     cancel,
		dispatcher: dispatcher,
		outputs:    make(map[string]chan processedResult, len(descriptors)),
	}

	for _, desc := range descriptors {
		reg := table.Lookup(desc.ID)
		if reg == nil {
			continue
		}
		inbox := dispatcher.Bind(desc.ID, reg)
		out := make(chan processedResult, 32)
		hub.outputs[desc.ID] = out
		go hub.runWorker(reg, inbox, out)
	}

	return hub
}

func (h *processorHub) Close() {
	if h == nil {
		return
	}
	h.cancel()
}

func (h *processorHub) Dispatch(messageTypeID string, raw []byte) processedResult {
	if h == nil {
		return processedResult{err: errs.New("", errs.CodeInvalid, errs.WithMessage("processor hub not initialized"))}
	}
	output, ok := h.outputs[messageTypeID]
	if !ok {
		return processedResult{err: errs.New("", errs.CodeInvalid, errs.WithMessage("message type not registered"))}
	}
	if err := h.dispatcher.Dispatch(messageTypeID, raw); err != nil {
		return processedResult{err: err}
	}
	select {
	case res := <-output:
		return res
	case <-h.ctx.Done():
		return processedResult{err: context.Canceled}
	}
}

func (h *processorHub) runWorker(reg *bnwsrouting.ProcessorRegistration, inbox <-chan []byte, output chan<- processedResult) {
	for {
		select {
		case <-h.ctx.Done():
			return
		case raw, ok := <-inbox:
			if !ok {
				return
			}
			msgIface, err := reg.Processor.Process(h.ctx, raw)
			if err != nil {
				select {
				case output <- processedResult{err: err}:
				case <-h.ctx.Done():
				}
				continue
			}
			routed, ok := msgIface.(*corestreams.RoutedMessage)
			if !ok {
				err := errs.New("", errs.CodeExchange, errs.WithMessage("processor returned unexpected type"))
				select {
				case output <- processedResult{err: err}:
				case <-h.ctx.Done():
				}
				continue
			}
			finalizeRoutedMessage(routed)
			select {
			case output <- processedResult{msg: routed}:
			case <-h.ctx.Done():
				return
			}
		}
	}
}

func finalizeRoutedMessage(msg *corestreams.RoutedMessage) {
	if msg == nil {
		return
	}
	switch evt := msg.Parsed.(type) {
	case *corestreams.TradeEvent:
		msg.At = evt.Time
	case *corestreams.TickerEvent:
		msg.At = evt.Time
	case *DepthDelta:
		msg.At = evt.EventTime
	case *corestreams.OrderEvent:
		msg.At = evt.Time
	case *corestreams.BalanceEvent:
		if len(evt.Balances) > 0 {
			msg.At = evt.Balances[0].Time
		}
	}
	if msg.At.IsZero() {
		msg.At = time.Now()
	}
}
