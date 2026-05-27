package eventbus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/domain/outboxstore"
	"github.com/coachpo/meltica/internal/domain/schema"
	"github.com/coachpo/meltica/internal/infra/pool"
)

func TestNewDurableBusReturnsInnerWhenStoreNil(t *testing.T) {
	inner := &stubBus{}
	wrapped := NewDurableBus(inner, nil)
	if wrapped != inner {
		t.Fatalf("expected original bus when store nil")
	}
}

func TestDurableBusPublishPersistsAndMarksDelivered(t *testing.T) {
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled())
	if bus == nil {
		t.Fatalf("expected durable bus instance")
	}
	event := &schema.Event{
		EventID:     "evt-1",
		Provider:    "binance",
		Symbol:      "BTCUSDT",
		Type:        schema.EventTypeTrade,
		SeqProvider: 42,
	}
	if err := bus.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if len(inner.published) != 1 {
		t.Fatalf("expected publish delegation, got %d", len(inner.published))
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected enqueued record, got %d", len(store.enqueued))
	}
	enqueued := store.enqueued[0]
	if enqueued.EventType != string(schema.EventTypeTrade) {
		t.Fatalf("expected enqueued trade type, got %s", enqueued.EventType)
	}
	if enqueued.Headers["eventId"] != "evt-1" {
		t.Fatalf("expected event id header to be persisted, got %v", enqueued.Headers["eventId"])
	}
	if len(store.delivered) != 1 {
		t.Fatalf("expected delivered marker, got %d", len(store.delivered))
	}
	if store.delivered[0] != 1 {
		t.Fatalf("expected delivered marker for row 1, got %d", store.delivered[0])
	}
	if len(store.failed) != 0 {
		t.Fatalf("unexpected failures: %v", store.failed)
	}
	bus.Close()
}

func TestDurableBusPublishRecordsFailure(t *testing.T) {
	pubErr := errors.New("publish failed")
	inner := &stubBus{publishErr: pubErr}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled())
	event := &schema.Event{
		EventID:     "evt-2",
		Provider:    "coinbase",
		Symbol:      "ETHUSD",
		Type:        schema.EventTypeTrade,
		SeqProvider: 7,
	}
	err := bus.Publish(context.Background(), event)
	if !errors.Is(err, pubErr) {
		t.Fatalf("expected publish error, got %v", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("expected failure recorded, got %d", len(store.failed))
	}
	if store.failed[0] != 1 {
		t.Fatalf("expected failure marker for row 1, got %d", store.failed[0])
	}
	if len(store.failedErrors) != 1 || !strings.Contains(store.failedErrors[0], pubErr.Error()) {
		t.Fatalf("expected stored failure to include %q, got %v", pubErr.Error(), store.failedErrors)
	}
	if len(store.delivered) != 0 {
		t.Fatalf("expected no delivered rows, got %d", len(store.delivered))
	}
	bus.Close()
}

func TestDurableBusReplayUsesEventPool(t *testing.T) {
	poolMgr := pool.NewPoolManager()
	err := poolMgr.RegisterPool("Event", 2, 2, func() any { return new(schema.Event) })
	if err != nil {
		t.Fatalf("register event pool: %v", err)
	}
	inner := NewMemoryBus(MemoryConfig{BufferSize: 4, FanoutWorkers: 1, Pools: poolMgr})
	store := &fakeOutboxStore{}
	wrapped := NewDurableBus(inner, store, WithReplayDisabled())
	durable, ok := wrapped.(*DurableBus)
	if !ok {
		t.Fatalf("expected durable bus instance")
	}
	durable.replayCtx = context.Background()
	evt := &schema.Event{
		EventID:  "evt-3",
		Provider: "binance",
		Symbol:   "BTCUSDT",
		Type:     schema.EventTypeTrade,
	}
	payload, err := eventToJSON(evt)
	if err != nil {
		t.Fatalf("eventToMap failed: %v", err)
	}
	store.pending = append(store.pending, outboxstore.EventRecord{ID: 1, EventType: string(evt.Type), Payload: payload})

	durable.replayPendingEvents()

	if len(store.delivered) != 1 {
		t.Fatalf("expected delivered record, got %d", len(store.delivered))
	}
	inner.Close()
}

func TestDurableBusReplayPublishesClaimedRowsInClaimOrder(t *testing.T) {
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	lease := 45 * time.Second
	bus := NewDurableBus(inner, store, WithReplayDisabled(), WithReplayBatchSize(2), WithReplayLease(lease))
	defer bus.Close()
	durable, ok := bus.(*DurableBus)
	if !ok {
		t.Fatalf("expected durable bus implementation")
	}
	durable.replayCtx = context.Background()
	store.pending = []outboxstore.EventRecord{
		{ID: 101, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "first")},
		{ID: 102, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "second")},
		{ID: 103, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "third")},
	}

	durable.replayPendingEvents()

	requirePublishedEventIDs(t, inner.published, []string{"first", "second"})
	requireInt64s(t, "delivered rows", store.delivered, []int64{101, 102})
	if len(store.failed) != 0 {
		t.Fatalf("unexpected failed rows: %v", store.failed)
	}
	if len(store.pending) != 1 || store.pending[0].ID != 103 {
		t.Fatalf("expected row 103 to remain pending for the next replay, got %+v", store.pending)
	}
	if len(store.claimPendingLimits) != 1 || store.claimPendingLimits[0] != 2 {
		t.Fatalf("expected ClaimPending to receive replay batch size 2, got %v", store.claimPendingLimits)
	}
	if len(store.claimPendingLeases) != 1 || store.claimPendingLeases[0] != lease {
		t.Fatalf("expected ClaimPending to receive replay lease %s, got %v", lease, store.claimPendingLeases)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("replay should mark rows, not delete them: %v", store.deleted)
	}

	durable.replayPendingEvents()

	requirePublishedEventIDs(t, inner.published, []string{"first", "second", "third"})
	requireInt64s(t, "delivered rows", store.delivered, []int64{101, 102, 103})
	if len(store.pending) != 0 {
		t.Fatalf("expected all pending rows to be drained after second replay, got %+v", store.pending)
	}
}

func TestDurableBusReplayMarksPendingRowsByOutcome(t *testing.T) {
	publishErr := errors.New("inner replay failed")
	inner := &stubBus{publishErrByEventID: map[string]error{"publish-fails": publishErr}}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled())
	defer bus.Close()
	durable, ok := bus.(*DurableBus)
	if !ok {
		t.Fatalf("expected durable bus implementation")
	}
	durable.replayCtx = context.Background()
	store.pending = []outboxstore.EventRecord{
		{ID: 201, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "deliver-me")},
		{ID: 202, EventType: string(schema.EventTypeTrade), Payload: json.RawMessage(`{`)},
		{ID: 203, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "publish-fails")},
		{ID: 204, EventType: string(schema.EventTypeTrade), Payload: durableReplayPayload(t, "deliver-after-failure")},
	}

	durable.replayPendingEvents()

	requirePublishedEventIDs(t, inner.published, []string{"deliver-me", "deliver-after-failure"})
	requireInt64s(t, "delivered rows", store.delivered, []int64{201, 204})
	requireInt64s(t, "failed rows", store.failed, []int64{202, 203})
	if len(store.failedErrors) != 2 {
		t.Fatalf("expected two failure messages, got %v", store.failedErrors)
	}
	if !strings.Contains(store.failedErrors[0], "unmarshal payload") {
		t.Fatalf("expected decode failure for row 202, got %q", store.failedErrors[0])
	}
	if !strings.Contains(store.failedErrors[1], publishErr.Error()) {
		t.Fatalf("expected publish failure for row 203, got %q", store.failedErrors[1])
	}
	if len(store.deleted) != 0 {
		t.Fatalf("replay should not delete failed or delivered rows directly: %v", store.deleted)
	}
}

func durableReplayPayload(t *testing.T, eventID string) json.RawMessage {
	t.Helper()
	raw, err := eventToJSON(&schema.Event{
		EventID:     eventID,
		Provider:    "binance",
		Symbol:      "BTCUSDT",
		Type:        schema.EventTypeTrade,
		SeqProvider: 1,
	})
	if err != nil {
		t.Fatalf("encode replay payload %s: %v", eventID, err)
	}
	return raw
}

func requirePublishedEventIDs(t *testing.T, got []*schema.Event, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected published event IDs %v, got %d events", want, len(got))
	}
	for i, wantID := range want {
		if got[i] == nil {
			t.Fatalf("published event %d is nil", i)
		}
		if got[i].EventID != wantID {
			t.Fatalf("published event %d: want %s got %s", i, wantID, got[i].EventID)
		}
	}
}

func requireInt64s(t *testing.T, name string, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: want %v got %v", name, want, got)
	}
	for i, wantValue := range want {
		if got[i] != wantValue {
			t.Fatalf("%s[%d]: want %d got %d (all values: %v)", name, i, wantValue, got[i], got)
		}
	}
}

type stubBus struct {
	published           []*schema.Event
	publishErr          error
	publishErrByEventID map[string]error
}

func (s *stubBus) Publish(_ context.Context, evt *schema.Event) error {
	if evt != nil && s.publishErrByEventID != nil {
		if err := s.publishErrByEventID[evt.EventID]; err != nil {
			return err
		}
	}
	if s.publishErr != nil {
		return s.publishErr
	}
	s.published = append(s.published, evt)
	return nil
}

func (*stubBus) Subscribe(context.Context, schema.EventType) (SubscriptionID, <-chan *schema.Event, error) {
	return "stub", make(chan *schema.Event), nil
}

func (*stubBus) Unsubscribe(SubscriptionID) {}

func (s *stubBus) Close() {}

type fakeOutboxStore struct {
	nextID             int64
	enqueued           []outboxstore.Event
	delivered          []int64
	failed             []int64
	failedErrors       []string
	pending            []outboxstore.EventRecord
	claimPendingLimits []int
	claimPendingLeases []time.Duration
	deleted            []int64
}

func (s *fakeOutboxStore) Enqueue(_ context.Context, evt outboxstore.Event) (outboxstore.EventRecord, error) {
	s.nextID++
	s.enqueued = append(s.enqueued, evt)
	payload := json.RawMessage(append([]byte(nil), evt.Payload...))
	record := outboxstore.EventRecord{ID: s.nextID, Payload: payload, EventType: evt.EventType}
	s.pending = append(s.pending, record)
	return record, nil
}

func (s *fakeOutboxStore) ClaimPending(_ context.Context, limit int, lease time.Duration) ([]outboxstore.EventRecord, error) {
	s.claimPendingLimits = append(s.claimPendingLimits, limit)
	s.claimPendingLeases = append(s.claimPendingLeases, lease)
	if len(s.pending) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > len(s.pending) {
		limit = len(s.pending)
	}
	batch := append([]outboxstore.EventRecord(nil), s.pending[:limit]...)
	s.pending = append([]outboxstore.EventRecord(nil), s.pending[limit:]...)
	for i := range batch {
		batch[i].Status = "processing"
		batch[i].Attempts++
	}
	return batch, nil
}

func (s *fakeOutboxStore) MarkDelivered(_ context.Context, id int64) error {
	s.delivered = append(s.delivered, id)
	return nil
}

func (s *fakeOutboxStore) MarkFailed(_ context.Context, id int64, lastError string) error {
	s.failed = append(s.failed, id)
	s.failedErrors = append(s.failedErrors, lastError)
	return nil
}

func (s *fakeOutboxStore) Delete(_ context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func TestDurableBusReplayPreservesSequenceAndPayload(t *testing.T) {
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled())
	durable, ok := bus.(*DurableBus)
	if !ok {
		t.Fatalf("expected durable bus implementation")
	}
	durable.replayCtx = context.Background()

	event := &schema.Event{
		EventID:     "evt-big",
		Provider:    "binance",
		Symbol:      "BTCUSDT",
		Type:        schema.EventTypeTrade,
		SeqProvider: 9007199254740995, // > 2^53 to catch float rounding
		Payload: schema.TradePayload{
			TradeID:  "t1",
			Side:     schema.TradeSideBuy,
			Price:    "123.45",
			Quantity: "1.0",
		},
	}
	raw, err := eventToJSON(event)
	if err != nil {
		t.Fatalf("encode event: %v", err)
	}
	store.pending = []outboxstore.EventRecord{{ID: 42, EventType: string(event.Type), Payload: raw}}

	durable.replayPendingEvents()

	if len(inner.published) != 1 {
		t.Fatalf("expected replayed publish, got %d", len(inner.published))
	}
	replayed := inner.published[0]
	if replayed.SeqProvider != event.SeqProvider {
		t.Fatalf("seq provider mismatch: want %d got %d", event.SeqProvider, replayed.SeqProvider)
	}
	payload, ok := replayed.Payload.(schema.TradePayload)
	if !ok {
		t.Fatalf("expected TradePayload, got %T", replayed.Payload)
	}
	if payload.TradeID != "t1" {
		t.Fatalf("expected payload trade id t1, got %s", payload.TradeID)
	}
}

func TestDurableBusPublishExtensionPayloadRoundTrip(t *testing.T) {
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled(), WithExtensionPayloadCapBytes(1024))
	ext := &schema.Event{
		EventID:  "ext-1",
		Provider: "test",
		Type:     schema.ExtensionEventType,
		Payload: map[string]any{
			"custom": map[string]any{"value": "alpha"},
		},
	}
	if err := bus.Publish(context.Background(), ext); err != nil {
		t.Fatalf("publish extension event: %v", err)
	}
	if len(store.enqueued) != 1 {
		t.Fatalf("expected enqueued record, got %d", len(store.enqueued))
	}
	if len(inner.published) != 1 {
		t.Fatalf("expected publish delegation, got %d", len(inner.published))
	}
	payload, ok := inner.published[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %T", inner.published[0].Payload)
	}
	nested, ok := payload["custom"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested custom payload, got %T", payload["custom"])
	}
	if value := nested["value"]; value != "alpha" {
		t.Fatalf("expected nested value alpha, got %v", value)
	}
}

func TestDurableBusPublishExtensionPayloadOverCap(t *testing.T) {
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled(), WithExtensionPayloadCapBytes(16))
	tooLarge := &schema.Event{
		EventID:  "ext-big",
		Provider: "test",
		Type:     schema.ExtensionEventType,
		Payload:  map[string]any{"data": strings.Repeat("x", 128)},
	}
	if err := bus.Publish(context.Background(), tooLarge); err == nil {
		t.Fatal("expected error for extension payload exceeding cap")
	}
	if len(store.enqueued) != 0 {
		t.Fatalf("expected no enqueued records, got %d", len(store.enqueued))
	}
	if len(inner.published) != 0 {
		t.Fatalf("expected no inner publishes, got %d", len(inner.published))
	}
}

func TestDurableBusPublishExtensionPayloadOverCapReturnsEventToPool(t *testing.T) {
	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", 1, 0, func() any { return new(schema.Event) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}
	inner := &stubBus{}
	store := &fakeOutboxStore{}
	bus := NewDurableBus(inner, store, WithReplayDisabled(), WithDurablePoolManager(poolMgr), WithExtensionPayloadCapBytes(16))
	ctx := context.Background()
	evt, err := poolMgr.BorrowEventInst(ctx)
	if err != nil {
		t.Fatalf("borrow event: %v", err)
	}
	evt.EventID = "ext-leak"
	evt.Provider = "test"
	evt.Symbol = "BTC-USDT"
	evt.Type = schema.ExtensionEventType
	evt.Payload = map[string]any{"data": strings.Repeat("x", 128)}

	if err := bus.Publish(ctx, evt); err == nil {
		t.Fatal("expected error for extension payload exceeding cap")
	}
	reclaimed, ok, err := poolMgr.TryBorrowEventInst()
	if err != nil {
		t.Fatalf("try borrow event: %v", err)
	}
	if !ok || reclaimed == nil {
		t.Fatalf("expected event to be returned to pool, ok=%t reclaimed=%v", ok, reclaimed)
	}
	poolMgr.ReturnEventInst(reclaimed)
	if err := poolMgr.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown pool: %v", err)
	}
}
