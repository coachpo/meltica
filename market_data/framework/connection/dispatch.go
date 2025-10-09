package connection

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/handler"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

type dispatcher struct {
	engine    *Engine
	session   *framework.ConnectionSession
	routes    map[string][]*bindingRuntime
	bindings  []*bindingRuntime
	errorSink func(error)
	emitter   *telemetry.Emitter
}

type bindingRuntime struct {
	binding handler.Binding
	handler stream.Handler
	limit   chan struct{}
}

type dispatchSummary struct {
	Invocations int
	Errors      int
	Dropped     bool
	Latency     time.Duration
}

type invocationResult struct {
	binding handler.Binding
	outcome stream.Outcome
	err     error
	latency time.Duration
}

func newDispatcher(engine *Engine, session *framework.ConnectionSession, opts DialOptions) (*dispatcher, error) {
	if engine == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("engine not configured"))
	}
	if session == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("session not configured"))
	}
	bindings := opts.Bindings
	if len(bindings) == 0 {
		active := engine.reg.Active()
		bindings = make([]handler.Binding, 0, len(active))
		for _, reg := range active {
			bindings = append(bindings, handler.Binding{
				Name:           reg.Name,
				Channels:       append([]string(nil), reg.Channels...),
				MaxConcurrency: 1,
			})
		}
	}
	if len(bindings) == 0 {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("no handler bindings provided"))
	}
	d := &dispatcher{
		engine:   engine,
		session:  session,
		routes:   make(map[string][]*bindingRuntime),
		bindings: make([]*bindingRuntime, 0, len(bindings)),
	}
	meta := mergeMetadata(session.Metadata(), opts.Metadata)
	for _, cfg := range bindings {
		runtime, err := d.prepareBinding(cfg, meta, opts.AuthToken)
		if err != nil {
			return nil, err
		}
		d.bindings = append(d.bindings, runtime)
		for _, channel := range cfg.Channels {
			key := normalizeChannel(channel)
			if key == "" {
				continue
			}
			d.routes[key] = append(d.routes[key], runtime)
		}
	}
	return d, nil
}

func (d *dispatcher) setErrorSink(fn func(error)) {
	if fn == nil {
		return
	}
	d.errorSink = fn
}

func (d *dispatcher) setEmitter(emitter *telemetry.Emitter) {
	d.emitter = emitter
}

func (d *dispatcher) prepareBinding(binding handler.Binding, metadata map[string]string, token string) (*bindingRuntime, error) {
	name := strings.TrimSpace(binding.Name)
	if name == "" {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("handler name is required"))
	}
	registration, ok := d.engine.reg.Resolve(name, "")
	if !ok {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage(fmt.Sprintf("handler %s is not registered", name)))
	}
	instance := registration.Instantiate()
	if instance == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage(fmt.Sprintf("handler %s factory returned nil", name)))
	}
	sessionMiddleware := []stream.Middleware{
		handler.WithSession(d.session),
		handler.WithMetadata(metadata),
		handler.WithAuthToken(token),
		handler.WithBinding(binding),
	}
	wrapped := handler.Chain(instance, sessionMiddleware...)
	concurrency := binding.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	return &bindingRuntime{
		binding: binding,
		handler: wrapped,
		limit:   make(chan struct{}, concurrency),
	}, nil
}

func (d *dispatcher) Dispatch(ctx context.Context, env *framework.MessageEnvelope) dispatchSummary {
	summary := dispatchSummary{}
	if d == nil || env == nil {
		return summary
	}
	targets := d.resolveTargets(env)
	if len(targets) == 0 {
		return summary
	}
	start := time.Now()
	for _, runtime := range targets {
		outcome, latency, err := runtime.invoke(ctx, env)
		result := invocationResult{binding: runtime.binding, outcome: outcome, err: err, latency: latency}
		summary.Invocations++
		if err != nil {
			summary.Errors++
			d.emitError(err)
			d.emitEvent(result, env, true, false)
			continue
		}
		errored, drop := d.handleOutcome(result, env)
		if errored {
			summary.Errors++
		}
		d.emitEvent(result, env, errored, drop)
		if drop {
			summary.Dropped = true
			break
		}
	}
	summary.Latency = time.Since(start)
	return summary
}

func (d *dispatcher) resolveTargets(env *framework.MessageEnvelope) []*bindingRuntime {
	channel := extractChannel(env)
	if channel != "" {
		if routes := d.routes[channel]; len(routes) > 0 {
			return routes
		}
	}
	return d.bindings
}

func (d *dispatcher) handleOutcome(result invocationResult, env *framework.MessageEnvelope) (bool, bool) {
	outcome := result.outcome
	if outcome == nil {
		return false, false
	}
	switch outcome.Type() {
	case stream.OutcomeTransform:
		if payload := outcome.Payload(); payload != nil {
			env.SetDecoded(payload)
		}
		return false, false
	case stream.OutcomeError:
		if err := outcome.Error(); err != nil {
			d.emitError(err)
		}
		return true, false
	case stream.OutcomeDrop:
		return false, true
	default:
		return false, false
	}
}

func (d *dispatcher) emitError(err error) {
	if err == nil || d == nil || d.errorSink == nil {
		return
	}
	d.errorSink(err)
}

func (d *dispatcher) emitEvent(result invocationResult, env *framework.MessageEnvelope, errored, dropped bool) {
	if d == nil || d.emitter == nil {
		return
	}
	kind := telemetry.EventMessageProcessed
	if errored {
		kind = telemetry.EventHandlerError
	}
	if dropped {
		kind = telemetry.EventMessageProcessed
	}
	event := telemetry.Event{
		Kind:      kind,
		SessionID: d.session.SessionID,
		Handler:   result.binding.Name,
		Latency:   result.latency,
		Metadata:  map[string]string{"channels": strings.Join(result.binding.Channels, ",")},
	}
	if result.err != nil {
		event.Error = result.err
	}
	if result.outcome != nil {
		event.Outcome = result.outcome.Type()
		if event.Error == nil {
			event.Error = result.outcome.Error()
		}
	}
	if dropped {
		event.Outcome = stream.OutcomeDrop
	}
	if event.Outcome == "" && !errored && !dropped {
		event.Outcome = stream.OutcomeAck
	}
	d.emitter.Emit(event)
}

func (runtime *bindingRuntime) invoke(ctx context.Context, env *framework.MessageEnvelope) (stream.Outcome, time.Duration, error) {
	if runtime == nil || runtime.handler == nil {
		return nil, 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case runtime.limit <- struct{}{}:
		defer func() { <-runtime.limit }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
	start := time.Now()
	outcome, err := runtime.handler.Handle(ctx, env)
	return outcome, time.Since(start), err
}

func extractChannel(env *framework.MessageEnvelope) string {
	if env == nil {
		return ""
	}
	decoded := env.Decoded()
	switch value := decoded.(type) {
	case map[string]any:
		return normalizeChannel(extractFromMap(value))
	case map[string]string:
		return normalizeChannel(extractFromStringMap(value))
	case nil:
		return ""
	default:
		return normalizeChannel(extractFromStruct(value))
	}
}

func extractFromMap(payload map[string]any) string {
	for _, key := range []string{"channel", "symbol", "topic"} {
		if val, ok := findInAnyMap(payload, key); ok {
			if str := toString(val); str != "" {
				return str
			}
		}
	}
	return ""
}

func extractFromStringMap(payload map[string]string) string {
	for _, key := range []string{"channel", "symbol", "topic"} {
		if val, ok := findInStringMap(payload, key); ok && val != "" {
			return val
		}
	}
	return ""
}

func findInAnyMap(payload map[string]any, target string) (any, bool) {
	for key, val := range payload {
		if strings.EqualFold(key, target) {
			return val, true
		}
	}
	return nil, false
}

func findInStringMap(payload map[string]string, target string) (string, bool) {
	for key, val := range payload {
		if strings.EqualFold(key, target) {
			return val, true
		}
	}
	return "", false
}

func extractFromStruct(value any) string {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		for _, field := range []string{"Channel", "Symbol", "Topic"} {
			f := rv.FieldByName(field)
			if f.IsValid() && f.CanInterface() {
				if str := toString(f.Interface()); str != "" {
					return str
				}
			}
		}
	}
	type channelProvider interface {
		Channel() string
	}
	if provider, ok := value.(channelProvider); ok {
		return provider.Channel()
	}
	return ""
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	}
	return ""
}

func normalizeChannel(channel string) string {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed)
}
