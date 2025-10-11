package wsrouting

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
)

func TestDetectionStrategyStringAndValidation(t *testing.T) {
	require.Equal(t, "field", DetectionStrategyFieldBased.String())
	require.Equal(t, "schema", DetectionStrategySchemaBased.String())
	require.Equal(t, "metadata", DetectionStrategyMetadataBased.String())
	require.Equal(t, "unknown", DetectionStrategy(99).String())

	require.True(t, DetectionStrategyFieldBased.valid())
	require.False(t, DetectionStrategy(42).valid())
}

func TestDetectionRuleValidateBranches(t *testing.T) {
	base := DetectionRule{Strategy: DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "trade"}
	require.NoError(t, base.Validate())

	_, err := NewFieldDetectionStrategy(base, "binance.trade")
	require.NoError(t, err)

	_, err = NewFieldDetectionStrategy(DetectionRule{Strategy: DetectionStrategyFieldBased, FieldPath: "", ExpectedValue: "trade"}, "binance.trade")
	require.Error(t, err)
	require.Error(t, DetectionRule{Strategy: DetectionStrategySchemaBased}.Validate())
	require.Error(t, DetectionRule{Strategy: DetectionStrategyMetadataBased, FieldPath: "extra"}.Validate())

	strat, err := NewFieldDetectionStrategy(base, "binance.trade")
	require.NoError(t, err)
	match, detectErr := strat.Detect([]byte(`{"e":"trade"}`))
	require.NoError(t, detectErr)
	require.Equal(t, "binance.trade", match)

	_, detectErr = strat.Detect([]byte(`{"e":"aggTrade"}`))
	require.Error(t, detectErr)
	require.True(t, isNoMatchError(detectErr))

	_, detectErr = strat.Detect([]byte(`{"e":true}`))
	require.Error(t, detectErr)
	require.False(t, isNoMatchError(detectErr))

	require.Equal(t, []string{"root", "child"}, splitPath(" root . child "))
	require.Empty(t, splitPath("  "))
}

func TestMessageTypeDescriptorValidate(t *testing.T) {
	desc := MessageTypeDescriptor{
		ID:             "binance.trade",
		DetectionRules: []DetectionRule{{Strategy: DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "trade"}},
		ProcessorRef:   "trade",
		SchemaVersion:  "v1",
	}
	require.NoError(t, desc.Validate())

	desc.ID = "Trade"
	require.Error(t, desc.Validate())
	desc.ID = ""
	require.Error(t, desc.Validate())
}

func TestProcessorRegistrationValidate(t *testing.T) {
	proc := &testProcessor{id: "binance.trade"}
	reg := ProcessorRegistration{MessageTypeID: "binance.trade", Processor: proc, Status: ProcessorStatusInitializing}
	require.NoError(t, reg.Validate())

	reg.Status = ProcessorStatusUnavailable
	require.Error(t, reg.Validate())
	reg.InitError = errors.New("boom")
	require.NoError(t, reg.Validate())
	reg.Status = ProcessorStatusAvailable
	require.Error(t, reg.Validate())
}

func TestRoutingTableRegisterDetectAndMetrics(t *testing.T) {
	rt := NewRoutingTable()
	proc := &testProcessor{id: "binance.trade"}
	desc := &MessageTypeDescriptor{
		ID:             "binance.trade",
		DetectionRules: []DetectionRule{{Strategy: DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "trade", Priority: 1}},
		ProcessorRef:   "trade",
	}
	require.NoError(t, rt.Register(desc, proc))

	defaultProc := &testProcessor{id: "default"}
	require.NoError(t, rt.SetDefault(defaultProc))

	reg := rt.Lookup("binance.trade")
	require.NotNil(t, reg)
	require.Equal(t, ProcessorStatusAvailable, reg.Status)

	messageType, err := rt.Detect([]byte(`{"e":"trade"}`))
	require.NoError(t, err)
	require.Equal(t, "binance.trade", messageType)

	metrics := rt.GetMetrics()
	require.NotNil(t, metrics)
	require.Greater(t, metrics.MessagesRouted["binance.trade"], uint64(0))

	_, err = rt.Detect([]byte(`{"e":"aggTrade"}`))
	require.Error(t, err)
	require.True(t, isNoMatchError(err))
}

func TestRoutingTableDetectErrors(t *testing.T) {
	rt := NewRoutingTable()
	proc := &testProcessor{id: "binance.trade"}
	desc := &MessageTypeDescriptor{
		ID:             "binance.trade",
		DetectionRules: []DetectionRule{{Strategy: DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "trade"}},
		ProcessorRef:   "trade",
	}
	require.NoError(t, rt.Register(desc, proc))

	_, err := rt.Detect(nil)
	require.Error(t, err)

	_, err = rt.Detect([]byte(`{"e":{`))
	require.Error(t, err)
	require.False(t, isNoMatchError(err))
}

func TestFlowControllerSend(t *testing.T) {
	metrics := NewRoutingMetrics()
	controller := NewFlowController(metrics)
	ch := make(chan []byte, 1)
	require.NoError(t, controller.Send(nil, "trade", ch, []byte("A")))
	require.Equal(t, uint64(0), metrics.BackpressureEvents)

	blocking := make(chan []byte)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		done <- controller.Send(ctx, "trade", blocking, []byte("B"))
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	err := <-done
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)
	require.Equal(t, uint64(1), metrics.BackpressureEvents)
	require.Zero(t, metrics.ChannelDepth["trade"])
}

func TestRouterDispatcherDispatch(t *testing.T) {
	metrics := NewRoutingMetrics()
	dispatcher := NewRouterDispatcher(context.Background(), metrics)
	reg := &ProcessorRegistration{MessageTypeID: "binance.trade", Processor: &testProcessor{id: "binance.trade"}, Status: ProcessorStatusAvailable}
	ch := dispatcher.Bind("binance.trade", reg)
	done := make(chan []byte, 1)
	go func() { done <- <-ch }()
	require.NoError(t, dispatcher.Dispatch("binance.trade", []byte("payload")))
	require.Equal(t, []byte("payload"), <-done)

	defaultCh := dispatcher.BindDefault(&ProcessorRegistration{MessageTypeID: "default", Processor: &testProcessor{id: "default"}, Status: ProcessorStatusAvailable})
	go func() { done <- <-defaultCh }()
	require.NoError(t, dispatcher.Dispatch("", []byte("fallback")))
	require.Equal(t, []byte("fallback"), <-done)

	dispatcher.flow = nil
	require.Error(t, dispatcher.Dispatch("binance.trade", []byte("x")))

	dispatcher = nil
	require.Error(t, dispatcher.Dispatch("binance.trade", []byte("y")))
}

func TestPipelineParserAndPublisher(t *testing.T) {
	var published atomic.Int32
	var lastType string
	sink := func(_ context.Context, messageType string, payload []byte) error {
		lastType = messageType
		if string(payload) == "error" {
			return errs.New("", errs.CodeExchange, errs.WithMessage("publish failed"))
		}
		published.Add(1)
		return nil
	}

	_, err := NewPipeline(nil)
	require.Error(t, err)

	pipeline, err := NewPipeline(sink)
	require.NoError(t, err)
	require.NotNil(t, pipeline.Parser())
	require.NotNil(t, pipeline.Publisher())

	msg, err := pipeline.Parser().Parse(context.Background(), []byte(`{"e":"trade"}`))
	require.NoError(t, err)
	require.Equal(t, "binance.trade", msg.Type)

	require.NoError(t, pipeline.Publisher()(context.Background(), msg))
	require.Equal(t, int32(1), published.Load())
	require.Equal(t, "binance.trade", lastType)

	msg.Payload["raw"] = "invalid"
	require.Error(t, pipeline.Publisher()(context.Background(), msg))

	_, err = pipeline.Parser().Parse(context.Background(), []byte(`{"stream":"test","data":{"e":"depthUpdate"}}`))
	require.NoError(t, err)

	_, err = pipeline.Parser().Parse(context.Background(), []byte(`{}`))
	require.Error(t, err)
	_, err = pipeline.Parser().Parse(context.Background(), []byte(`{"e":"unknown"}`))
	require.Error(t, err)
}

func TestMapEventToTypeAndHelpers(t *testing.T) {
	cases := map[string]string{
		"trade":                   tradeType,
		"depthUpdate":             orderBookType,
		"24hrTicker":              tickerType,
		"order_trade_update":      orderUpdateType,
		"balanceUpdate":           balanceUpdateType,
		"outboundAccountPosition": balanceUpdateType,
	}
	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			typeID, ok := mapEventToType(input)
			require.True(t, ok)
			require.Equal(t, expected, typeID)
		})
	}
	_, ok := mapEventToType("unknown")
	require.False(t, ok)

	fields, err := decodePayloadFields([]byte(`{"e":"trade"}`))
	require.NoError(t, err)
	require.Contains(t, fields, "e")

	payload, stream := unwrapCombinedPayload([]byte(`{"stream":"trade","data":{"e":"trade"}}`))
	require.Equal(t, []byte(`{"e":"trade"}`), payload)
	require.Equal(t, "trade", stream)

	event, err := eventType(payload)
	require.NoError(t, err)
	require.Equal(t, "trade", event)

	_, err = eventType([]byte(`{"e":1}`))
	require.Error(t, err)
}

func TestRoutingMetricsSnapshot(t *testing.T) {
	metrics := NewRoutingMetrics()
	metrics.RecordRoute("trade")
	metrics.RecordError()
	metrics.RecordProcessing("trade", 5*time.Millisecond, nil)
	metrics.RecordProcessing("trade", 10*time.Millisecond, errors.New("boom"))
	metrics.RecordBackpressureStart("trade")
	metrics.RecordBackpressureEnd("trade")

	snapshot := metrics.Snapshot()
	require.NotNil(t, snapshot)
	require.Equal(t, uint64(1), snapshot.RoutingErrors)
	require.Equal(t, metrics.ProcessorInvocations["trade"].Successes, snapshot.ProcessorInvocations["trade"].Successes)
	require.Equal(t, metrics.ConversionDurations["trade"].P95, snapshot.ConversionDurations["trade"].P95)
}

type testProcessor struct {
	id      string
	initErr error
}

func (p *testProcessor) Initialize(context.Context) error {
	return p.initErr
}

func (p *testProcessor) Process(context.Context, []byte) (interface{}, error) {
	return nil, nil
}

func (p *testProcessor) MessageTypeID() string {
	return p.id
}
