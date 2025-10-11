package bridge

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
	"github.com/coachpo/meltica/errs"
)

type layerBusinessAdapter struct {
	dispatcher Dispatcher
	mu         sync.RWMutex
	state      layers.BusinessState
}

var _ layers.Business = (*layerBusinessAdapter)(nil)

// NewBusinessAdapter wraps a Dispatcher and exposes a layers.Business interface for migration.
func NewBusinessAdapter(dispatcher Dispatcher) layers.Business {
	return &layerBusinessAdapter{
		dispatcher: dispatcher,
		state: layers.BusinessState{
			Status:     "initialized",
			Metrics:    make(map[string]any),
			LastUpdate: time.Now().UTC(),
		},
	}
}

func (a *layerBusinessAdapter) Process(ctx context.Context, msg layers.NormalizedMessage) (layers.BusinessResult, error) {
	result := layers.BusinessResult{Success: true, Data: msg.Data}
	a.mu.Lock()
	a.state.LastUpdate = time.Now().UTC()
	a.state.Status = "processed"
	a.mu.Unlock()
	return result, nil
}

func (a *layerBusinessAdapter) Validate(context.Context, layers.BusinessRequest) error {
	if a.dispatcher == nil {
		return errs.NotSupported("binance business adapter: dispatcher unavailable")
	}
	return nil
}

func (a *layerBusinessAdapter) GetState() layers.BusinessState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// AsLayerInterface exposes the wrapper as a layers.Business contract.
func (w *Wrapper) AsLayerInterface() layers.Business {
	return NewBusinessAdapter(w.dispatcher)
}
