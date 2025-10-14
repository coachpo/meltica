package conductor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

type providerStreams struct {
	events <-chan *schema.Event
	errs   <-chan error
}

type mergeRule struct {
	key            string
	providers      map[string]struct{}
	windowDuration time.Duration
	maxEvents      int
}

// MergeRuleDefinition describes a merge window keyed by symbol and canonical type.
type MergeRuleDefinition struct {
	Symbol         string
	EventType      schema.EventType
	Providers      []string
	WindowDuration time.Duration
	MaxEvents      int
}

func (def MergeRuleDefinition) key() string {
	symbol := strings.ToUpper(strings.TrimSpace(def.Symbol))
	evt := strings.ToUpper(string(def.EventType))
	if symbol == "" || evt == "" {
		return ""
	}
	return fmt.Sprintf("%s|%s", symbol, evt)
}

type mergeWindow struct {
	rule      *mergeRule
	openedAt  time.Time
	fragments map[string]*schema.Event
	count     int
}

type mergeState int

const (
	mergeStateNone mergeState = iota
	mergeStateStored
	mergeStateComplete
	mergeStateLate
)

func newMergeWindow(rule *mergeRule, openedAt time.Time) *mergeWindow {
	window := new(mergeWindow)
	window.rule = rule
	window.openedAt = openedAt
	window.fragments = make(map[string]*schema.Event, len(rule.providers))
	return window
}

func (w *mergeWindow) expired(now time.Time) bool {
	if w == nil || w.rule == nil {
		return false
	}
	dur := w.rule.windowDuration
	if dur <= 0 {
		dur = 10 * time.Second
	}
	return now.Sub(w.openedAt) >= dur
}

func (w *mergeWindow) add(evt *schema.Event, now time.Time) mergeState {
	if w == nil || w.rule == nil || evt == nil {
		return mergeStateNone
	}
	if len(w.rule.providers) > 0 {
		if _, ok := w.rule.providers[strings.ToLower(strings.TrimSpace(evt.Provider))]; !ok {
			return mergeStateLate
		}
	}
	if !evt.IngestTS.IsZero() && evt.IngestTS.Before(w.openedAt.Add(-1*time.Second)) {
		return mergeStateLate
	}
	w.fragments[strings.ToLower(evt.Provider)] = evt
	w.count++
	if len(w.rule.providers) > 0 && len(w.fragments) >= len(w.rule.providers) {
		return mergeStateComplete
	}
	if w.rule.maxEvents > 0 && w.count >= w.rule.maxEvents {
		return mergeStateComplete
	}
	return mergeStateStored
}

func (w *mergeWindow) fragmentsSlice() []*schema.Event {
	if w == nil {
		return nil
	}
	result := make([]*schema.Event, 0, len(w.fragments))
	for _, evt := range w.fragments {
		if evt != nil {
			result = append(result, evt)
		}
	}
	return result
}

// EventOrchestrator coordinates canonical event streams from providers.
type EventOrchestrator struct {
	mu        sync.Mutex
	providers map[string]providerStreams
	events    chan *schema.Event
	errs      chan error
	started   bool
	pools     *pool.PoolManager
	clock     func() time.Time
	mergeMu   sync.RWMutex
	rules     map[string]*mergeRule
	windowMu  sync.Mutex
	windows   map[string]*mergeWindow
	mergeSeq  uint64
}

// NewEventOrchestrator constructs an event orchestrator instance.
func NewEventOrchestrator() *EventOrchestrator {
	return NewEventOrchestratorWithPool(nil)
}

// NewEventOrchestratorWithPool constructs an event orchestrator instance that integrates with a pool manager.
func NewEventOrchestratorWithPool(pools *pool.PoolManager) *EventOrchestrator {
	orchestrator := new(EventOrchestrator)
	orchestrator.providers = make(map[string]providerStreams)
	orchestrator.events = make(chan *schema.Event, 128)
	orchestrator.errs = make(chan error, 8)
	orchestrator.pools = pools
	orchestrator.clock = time.Now
	orchestrator.rules = make(map[string]*mergeRule)
	orchestrator.windows = make(map[string]*mergeWindow)
	return orchestrator
}

// UpsertMergeRule registers or replaces a merge rule definition.
func (o *EventOrchestrator) UpsertMergeRule(def MergeRuleDefinition) {
	key := def.key()
	if key == "" {
		return
	}
	providers := make(map[string]struct{}, len(def.Providers))
	for _, provider := range def.Providers {
		if p := strings.ToLower(strings.TrimSpace(provider)); p != "" {
			providers[p] = struct{}{}
		}
	}
	if len(providers) == 0 && len(def.Providers) > 0 {
		for _, provider := range def.Providers {
			providers[strings.ToLower(strings.TrimSpace(provider))] = struct{}{}
		}
	}
	rule := &mergeRule{
		key:            key,
		providers:      providers,
		windowDuration: def.WindowDuration,
		maxEvents:      def.MaxEvents,
	}
	if rule.windowDuration <= 0 {
		rule.windowDuration = 10 * time.Second
	}
	if rule.maxEvents <= 0 {
		rule.maxEvents = 1000
	}
	o.mergeMu.Lock()
	o.rules[key] = rule
	o.mergeMu.Unlock()
	o.windowMu.Lock()
	delete(o.windows, key)
	o.windowMu.Unlock()
}

// RemoveMergeRule removes a merge rule by symbol and type.
func (o *EventOrchestrator) RemoveMergeRule(symbol string, typ schema.EventType) {
	key := mergeKey(symbol, typ)
	if key == "" {
		return
	}
	o.mergeMu.Lock()
	delete(o.rules, key)
	o.mergeMu.Unlock()
	o.windowMu.Lock()
	if window, ok := o.windows[key]; ok {
		for _, evt := range window.fragments {
			o.releaseEvent(evt)
		}
		delete(o.windows, key)
	}
	o.windowMu.Unlock()
}

func mergeKey(symbol string, typ schema.EventType) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	t := strings.ToUpper(string(typ))
	if s == "" || t == "" {
		return ""
	}
	return fmt.Sprintf("%s|%s", s, t)
}

// AddProvider registers a provider event stream with the orchestrator.
func (o *EventOrchestrator) AddProvider(name string, events <-chan *schema.Event, errs <-chan error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.started {
		return
	}
	if name == "" {
		name = "provider"
	}
	o.providers[name] = providerStreams{events: events, errs: errs}
}

// Events exposes the orchestrated canonical event stream.
func (o *EventOrchestrator) Events() <-chan *schema.Event {
	return o.events
}

// Errors exposes asynchronous orchestration errors.
func (o *EventOrchestrator) Errors() <-chan error {
	return o.errs
}

// Start runs the orchestrator loop until the context is cancelled.
func (o *EventOrchestrator) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("orchestrator requires context")
	}

	o.mu.Lock()
	if o.started {
		o.mu.Unlock()
		return errors.New("orchestrator already started")
	}
	o.started = true
	providers := make([]providerStreams, 0, len(o.providers))
	for _, streams := range o.providers {
		providers = append(providers, streams)
	}
	o.mu.Unlock()

	var wg sync.WaitGroup
	for _, streams := range providers {
		if streams.events != nil {
			wg.Add(1)
			go func(stream <-chan *schema.Event) {
				defer wg.Done()
				o.forwardEvents(ctx, stream)
			}(streams.events)
		}
		if streams.errs != nil {
			wg.Add(1)
			go func(errs <-chan error) {
				defer wg.Done()
				o.forwardErrors(ctx, errs)
			}(streams.errs)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
	case <-done:
	}

	<-done
	close(o.events)
	close(o.errs)
	return fmt.Errorf("orchestrator context: %w", ctx.Err())
}

func (o *EventOrchestrator) forwardEvents(ctx context.Context, stream <-chan *schema.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-stream:
			if !ok {
				return
			}
			o.handleEvent(ctx, evt)
		}
	}
}

func (o *EventOrchestrator) handleEvent(ctx context.Context, evt *schema.Event) {
	if evt == nil {
		return
	}
	if o.tryMerge(ctx, evt) {
		return
	}

	var (
		merged  *schema.MergedEvent
		release func()
	)

	if evt.MergeID != nil {
		var err error
		merged, release, err = o.acquireMergedEvent(ctx)
		if err != nil {
			o.emitError(fmt.Errorf("orchestrator: acquire merged event: %w", err))
			release = nil
		} else {
			o.populateMergedEvent(merged, evt)
		}
	}

	o.dispatchEvent(ctx, evt, release)
}

func (o *EventOrchestrator) tryMerge(ctx context.Context, evt *schema.Event) bool {
	rule := o.lookupRule(evt)
	if rule == nil {
		return false
	}
	now := o.now()
	key := rule.key
	var emit []*schema.Event
	var drop []*schema.Event

	o.windowMu.Lock()
	window := o.windows[key]
	if window != nil && window.expired(now) {
		fragments := window.fragmentsSlice()
		if len(rule.providers) == 0 || len(fragments) >= len(rule.providers) {
			emit = append(emit, fragments...)
		} else {
			drop = append(drop, fragments...)
		}
		delete(o.windows, key)
		window = nil
	}
	if window == nil {
		window = newMergeWindow(rule, now)
		o.windows[key] = window
	}
	state := window.add(evt, now)
	switch state {
	case mergeStateLate:
		drop = append(drop, evt)
	case mergeStateComplete:
		emit = append(emit, window.fragmentsSlice()...)
		delete(o.windows, key)
	}
	o.windowMu.Unlock()

	for _, d := range drop {
		o.releaseEvent(d)
	}
	if len(emit) == 0 {
		return state != mergeStateNone
	}
	mergeID := o.nextMergeID(key)
	for _, fragment := range emit {
		if fragment == nil {
			continue
		}
		o.assignMergeID(fragment, mergeID)
		o.dispatchEvent(ctx, fragment, nil)
	}
	return true
}

func (o *EventOrchestrator) lookupRule(evt *schema.Event) *mergeRule {
	if evt == nil {
		return nil
	}
	key := mergeKey(evt.Symbol, evt.Type)
	if key == "" {
		return nil
	}
	o.mergeMu.RLock()
	rule := o.rules[key]
	o.mergeMu.RUnlock()
	return rule
}

func (o *EventOrchestrator) assignMergeID(evt *schema.Event, mergeID string) {
	if evt == nil {
		return
	}
	if mergeID == "" {
		return
	}
	evt.MergeID = &mergeID
	if evt.EmitTS.IsZero() {
		evt.EmitTS = o.now()
	}
}

func (o *EventOrchestrator) nextMergeID(key string) string {
	seq := atomic.AddUint64(&o.mergeSeq, 1)
	return fmt.Sprintf("%s#%d", key, seq)
}

func (o *EventOrchestrator) now() time.Time {
	if o != nil && o.clock != nil {
		return o.clock().UTC()
	}
	return time.Now().UTC()
}

func (o *EventOrchestrator) releaseEvent(evt *schema.Event) {
	if evt == nil {
		return
	}
	if o.pools != nil {
		evt.Reset()
		o.pools.Put("CanonicalEvent", evt)
	}
}

func (o *EventOrchestrator) acquireMergedEvent(ctx context.Context) (*schema.MergedEvent, func(), error) {
	if o.pools == nil {
		merged := new(schema.MergedEvent)
		return merged, func() {}, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	getCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	obj, err := o.pools.Get(getCtx, "MergedEvent")
	cancel()
	if err != nil {
		return nil, nil, fmt.Errorf("acquire merged event: %w", err)
	}

	merged, ok := obj.(*schema.MergedEvent)
	if !ok {
		o.pools.Put("MergedEvent", obj)
		return nil, nil, fmt.Errorf("orchestrator: unexpected object type %T", obj)
	}

	release := func() {
		o.pools.Put("MergedEvent", merged)
	}
	return merged, release, nil
}

func (o *EventOrchestrator) populateMergedEvent(merged *schema.MergedEvent, evt *schema.Event) {
	if merged == nil || evt == nil {
		return
	}

	if evt.MergeID != nil {
		merged.MergeID = *evt.MergeID
	}
	merged.Symbol = evt.Symbol
	merged.EventType = evt.Type
	if !evt.IngestTS.IsZero() {
		merged.WindowOpen = evt.IngestTS.UnixNano()
	}
	if !evt.EmitTS.IsZero() {
		merged.WindowClose = evt.EmitTS.UnixNano()
	}
	merged.TraceID = evt.TraceID
	merged.Fragments = append(merged.Fragments, *evt)
	merged.IsComplete = true
}

func (o *EventOrchestrator) dispatchEvent(ctx context.Context, evt *schema.Event, release func()) {
	if release == nil {
		release = func() {}
	}

	select {
	case <-ctx.Done():
		release()
	case o.events <- evt:
		release()
	}
}

func (o *EventOrchestrator) forwardErrors(ctx context.Context, errs <-chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case o.errs <- err:
			default:
			}
		}
	}
}

func (o *EventOrchestrator) emitError(err error) {
	if err == nil {
		return
	}
	select {
	case o.errs <- err:
	default:
	}
}
