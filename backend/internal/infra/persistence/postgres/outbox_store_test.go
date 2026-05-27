package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	json "github.com/goccy/go-json"
	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/coachpo/meltica/internal/domain/outboxstore"
)

func TestOutboxStoreNilPool(t *testing.T) {
	store := NewOutboxStore(nil)
	ctx := context.Background()
	event := outboxstore.Event{
		AggregateType: "eventbus",
		AggregateID:   "evt-1",
		EventType:     "Trade",
		Payload:       json.RawMessage(`{"eventId":"evt-1"}`),
	}
	if _, err := store.Enqueue(ctx, event); err == nil {
		t.Fatalf("expected error when pool nil")
	}
	if _, err := store.ClaimPending(ctx, 1, time.Minute); err == nil {
		t.Fatalf("expected error when pool nil")
	}
	if err := store.MarkDelivered(ctx, 1); err == nil {
		t.Fatalf("expected error when pool nil")
	}
	if err := store.MarkFailed(ctx, 1, "error"); err == nil {
		t.Fatalf("expected error when pool nil")
	}
	if err := store.Delete(ctx, 1); err == nil {
		t.Fatalf("expected error when pool nil")
	}
}

func TestOutboxClaimPendingClaimsReadyRows(t *testing.T) {
	ctx := context.Background()
	store := newOutboxIntegrationStore(t, ctx)
	now := time.Now().Add(-time.Second)

	first := enqueueOutboxTestEvent(t, ctx, store, "first", now)
	second := enqueueOutboxTestEvent(t, ctx, store, "second", now.Add(time.Millisecond))
	enqueueOutboxTestEvent(t, ctx, store, "future", time.Now().Add(time.Hour))
	claimed, err := store.ClaimPending(ctx, 1, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim pending: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != first.ID {
		t.Fatalf("expected only first row claimed, got %+v", claimed)
	}
	if claimed[0].Status != "processing" || claimed[0].ClaimedAt == nil || claimed[0].Attempts != 1 {
		t.Fatalf("expected processing claim with claimed_at and attempt, got %+v", claimed[0])
	}

	claimed, err = store.ClaimPending(ctx, 10, 2*time.Minute)
	if err != nil {
		t.Fatalf("second claim pending: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != second.ID {
		t.Fatalf("expected only second ready row claimed, got %+v", claimed)
	}
	if err := store.MarkDelivered(ctx, first.ID); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}
	if err := store.MarkFailed(ctx, second.ID, "retry me"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	claimed, err = store.ClaimPending(ctx, 10, 2*time.Minute)
	if err != nil {
		t.Fatalf("claim after terminal/retry transitions: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("expected delivered rows and delayed retry rows to be excluded, got %+v", claimed)
	}
}

func TestOutboxClaimPendingDoesNotReopenDeliveredRows(t *testing.T) {
	ctx := context.Background()
	store := newOutboxIntegrationStore(t, ctx)
	record := enqueueOutboxTestEvent(t, ctx, store, "delivered-race", time.Now().Add(-time.Second))

	claimed, err := store.ClaimPending(ctx, 1, time.Minute)
	if err != nil {
		t.Fatalf("claim pending: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != record.ID {
		t.Fatalf("expected claimed row %d, got %+v", record.ID, claimed)
	}
	if err := store.MarkDelivered(ctx, record.ID); err != nil {
		t.Fatalf("mark delivered: %v", err)
	}
	if err := store.MarkFailed(ctx, record.ID, "stale replay failed after delivery"); err != nil {
		t.Fatalf("stale mark failed should be idempotent after delivery: %v", err)
	}

	claimed, err = store.ClaimPending(ctx, 1, time.Minute)
	if err != nil {
		t.Fatalf("claim after stale reset: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("delivered row was reopened for replay: %+v", claimed)
	}
	status, delivered, claimedAt, lastError := loadOutboxState(t, ctx, store, record.ID)
	if status != "delivered" || !delivered || claimedAt.Valid || lastError.Valid {
		t.Fatalf("expected delivered row to remain terminal and clean, got status=%s delivered=%t claimed_at=%v last_error=%v", status, delivered, claimedAt, lastError)
	}
}

func TestOutboxReclaimsStaleProcessingRows(t *testing.T) {
	ctx := context.Background()
	store := newOutboxIntegrationStore(t, ctx)
	record := enqueueOutboxTestEvent(t, ctx, store, "stale", time.Now().Add(-time.Second))

	claimed, err := store.ClaimPending(ctx, 1, time.Minute)
	if err != nil {
		t.Fatalf("initial claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != record.ID {
		t.Fatalf("expected initial claim for row %d, got %+v", record.ID, claimed)
	}
	claimed, err = store.ClaimPending(ctx, 1, time.Minute)
	if err != nil {
		t.Fatalf("fresh processing claim: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("fresh processing row should remain leased, got %+v", claimed)
	}

	_, err = store.pool.Exec(ctx, `
		UPDATE events_outbox
		SET claimed_at = NOW() - INTERVAL '5 minutes', available_at = NOW()
		WHERE id = $1
	`, record.ID)
	if err != nil {
		t.Fatalf("age processing claim: %v", err)
	}
	claimed, err = store.ClaimPending(ctx, 1, time.Minute)
	if err != nil {
		t.Fatalf("stale processing reclaim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != record.ID {
		t.Fatalf("expected stale processing row to be reclaimed, got %+v", claimed)
	}
	if claimed[0].Status != "processing" || claimed[0].ClaimedAt == nil || claimed[0].Attempts != 2 {
		t.Fatalf("expected reclaimed row to stay processing with incremented attempt, got %+v", claimed[0])
	}
}

func newOutboxIntegrationStore(t *testing.T, ctx context.Context) *OutboxStore {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		Env:          map[string]string{"POSTGRES_PASSWORD": "secret", "POSTGRES_USER": "postgres", "POSTGRES_DB": "meltica"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("postgres container unavailable: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://postgres:secret@%s:%s/meltica?sslmode=disable", host, port.Port())
	applyOutboxTestMigrations(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgx pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewOutboxStore(pool)
}

func applyOutboxTestMigrations(t *testing.T, dsn string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller lookup failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	sourceURL := fmt.Sprintf("file://%s", filepath.Join(root, "db", "migrations"))

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open sql connection: %v", err)
	}
	defer sqlDB.Close()
	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		t.Fatalf("postgres migration driver: %v", err)
	}
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		t.Fatalf("migration instance: %v", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate up: %v", err)
	}
}

func loadOutboxState(t *testing.T, ctx context.Context, store *OutboxStore, id int64) (string, bool, pgtype.Timestamptz, pgtype.Text) {
	t.Helper()
	var status string
	var delivered bool
	var claimedAt pgtype.Timestamptz
	var lastError pgtype.Text
	err := store.pool.QueryRow(ctx, `
		SELECT status, delivered, claimed_at, last_error
		FROM events_outbox
		WHERE id = $1
	`, id).Scan(&status, &delivered, &claimedAt, &lastError)
	if err != nil {
		t.Fatalf("load outbox state: %v", err)
	}
	return status, delivered, claimedAt, lastError
}

func enqueueOutboxTestEvent(t *testing.T, ctx context.Context, store *OutboxStore, id string, availableAt time.Time) outboxstore.EventRecord {
	t.Helper()
	payload := json.RawMessage(fmt.Sprintf(`{"eventId":%q}`, id))
	record, err := store.Enqueue(ctx, outboxstore.Event{
		AggregateType: "eventbus",
		AggregateID:   id,
		EventType:     "Trade",
		Payload:       payload,
		Headers:       map[string]any{"test": id},
		AvailableAt:   availableAt,
	})
	if err != nil {
		t.Fatalf("enqueue %s: %v", id, err)
	}
	if record.Status != "pending" || record.ClaimedAt != nil || record.Delivered {
		t.Fatalf("expected enqueued row to be pending and unclaimed, got %+v", record)
	}
	return record
}
