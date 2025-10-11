package processors

import (
	"context"
	"log"
	"strconv"

	"github.com/coachpo/meltica/market_data"
)

type DefaultProcessor struct{}

func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{}
}

func (p *DefaultProcessor) Initialize(ctx context.Context) error {
	return contextError(ctx)
}

func (p *DefaultProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	log.Printf("processors: default processor handling unrecognized message (%d bytes)", len(raw))
	clone := append([]byte(nil), raw...)
	metadata := map[string]string{
		"processor": p.MessageTypeID(),
	}
	if len(raw) > 0 {
		metadata["length"] = strconv.Itoa(len(raw))
	}
	return &market_data.RawPayload{Data: clone, Metadata: metadata}, nil
}

func (p *DefaultProcessor) MessageTypeID() string { return "default" }

var _ Processor = (*DefaultProcessor)(nil)
