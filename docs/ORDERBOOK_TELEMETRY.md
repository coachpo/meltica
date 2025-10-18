# Orderbook Telemetry

## Overview

The BookAssembler emits comprehensive telemetry to track orderbook health, gap detection, and recovery processes using OpenTelemetry metrics.

## Metrics

All metrics are emitted from the `orderbook` meter and include `symbol` attribute for per-symbol tracking.

### 1. `orderbook.gap.detected` (Counter)

**Description**: Number of sequence gaps detected requiring snapshot restart.

**Unit**: `{gap}`

**Attributes**:
- `symbol` (string): Trading pair symbol (e.g., "BTC-USDT")
- `first_seq` (int64): First sequence ID of the gap-triggering update
- `final_seq` (int64): Final sequence ID of the gap-triggering update
- `current_seq` (int64): Current sequence before gap
- `gap_size` (int64): Number of missing updates (first_seq - current_seq - 1)
- `recovery_action` (string): Always "snapshot_restart"

**When Emitted**: When `firstUpdateID > seq+1` indicating missing updates

**Example**:
```
orderbook.gap.detected{symbol="BTC-USDT",gap_size=2,recovery_action="snapshot_restart"} = 1
```

**Alerting Thresholds**:
- **Warning**: > 5 gaps/hour for a symbol
- **Critical**: > 20 gaps/hour for a symbol
- Indicates network issues or provider problems

---

### 2. `orderbook.buffer.size` (UpDownCounter)

**Description**: Number of updates currently buffered (cold start or recovery).

**Unit**: `{update}`

**Attributes**:
- `symbol` (string): Trading pair symbol
- `first_seq` (int64): Sequence of buffered update
- `final_seq` (int64): Sequence of buffered update
- `current_seq` (int64): Current book sequence

**When Emitted**: 
- `+1` when update buffered during cold start or recovery
- `-N` when buffer cleared after snapshot replay

**Example**:
```
orderbook.buffer.size{symbol="ETH-USDT"} = 15
```

**Alerting Thresholds**:
- **Warning**: > 100 buffered updates
- **Critical**: > 500 buffered updates
- Indicates snapshot fetch delays

---

### 3. `orderbook.update.stale` (Counter)

**Description**: Number of stale updates rejected (finalUpdateID <= current seq).

**Unit**: `{update}`

**Attributes**:
- `symbol` (string): Trading pair symbol
- `first_seq` (int64): Sequence of stale update
- `final_seq` (int64): Sequence of stale update
- `current_seq` (int64): Current book sequence

**When Emitted**: When `finalUpdateID <= seq` (update already processed)

**Example**:
```
orderbook.update.stale{symbol="XRP-USDT"} = 3
```

**Alerting Thresholds**:
- **Info**: Occasional stale updates are normal (< 10/hour)
- **Warning**: > 50 stale updates/hour
- Indicates out-of-order delivery or clock skew

---

### 4. `orderbook.snapshot.applied` (Counter)

**Description**: Number of snapshots applied (initial and periodic refresh).

**Unit**: `{snapshot}`

**Attributes**:
- `symbol` (string): Trading pair symbol
- `sequence` (int64): Snapshot's lastUpdateId

**When Emitted**: Every time `ApplySnapshot()` is called

**Example**:
```
orderbook.snapshot.applied{symbol="BTC-USDT"} = 120
```

**Expected Rate**: 
- With 1m refresh: ~60 snapshots/hour per symbol
- With 30s refresh: ~120 snapshots/hour per symbol

---

### 5. `orderbook.updates.replayed` (Counter)

**Description**: Number of buffered updates successfully replayed after snapshot.

**Unit**: `{update}`

**Attributes**:
- `symbol` (string): Trading pair symbol
- `sequence` (int64): Final sequence after replay
- `buffered` (int): Total updates that were buffered
- `valid` (int): Number actually replayed (after filtering stale)

**When Emitted**: After snapshot when buffer contains valid updates

**Example**:
```
orderbook.updates.replayed{symbol="ETH-USDT",buffered=5,valid=3} = 3
```

**Interpretation**:
- `valid == buffered`: All buffered updates were fresh
- `valid < buffered`: Some buffered updates were stale (already in snapshot)

---

### 6. `orderbook.coldstart.duration` (Histogram)

**Description**: Time from first update to snapshot ready (milliseconds).

**Unit**: `ms`

**Attributes**:
- `symbol` (string): Trading pair symbol
- `sequence` (int64): Final sequence after cold start

**When Emitted**: When first snapshot is applied after cold start

**Example**:
```
orderbook.coldstart.duration{symbol="BTC-USDT",p50=250,p95=500,p99=1000}
```

**Alerting Thresholds**:
- **Good**: p95 < 500ms
- **Warning**: p95 > 1000ms
- **Critical**: p95 > 5000ms
- Indicates snapshot fetch latency

---

## Monitoring Queries

### Grafana/Prometheus Queries

#### Gap Detection Rate
```promql
rate(orderbook_gap_detected_total[5m])
```

#### Current Buffer Sizes
```promql
orderbook_buffer_size
```

#### Stale Update Rate
```promql
rate(orderbook_update_stale_total[5m])
```

#### Snapshot Application Rate
```promql
rate(orderbook_snapshot_applied_total[5m])
```

#### Cold Start Duration (p95)
```promql
histogram_quantile(0.95, rate(orderbook_coldstart_duration_bucket[5m]))
```

#### Updates Replayed per Symbol
```promql
sum by (symbol) (orderbook_updates_replayed_total)
```

---

## Dashboard Layout

### Recommended Panels

1. **Gap Detection Overview**
   - Metric: `orderbook.gap.detected`
   - Visualization: Time series line chart
   - Group by: symbol
   - Alert if > 5 gaps/hour

2. **Buffer Size Heatmap**
   - Metric: `orderbook.buffer.size`
   - Visualization: Heatmap
   - Y-axis: symbol
   - Color: buffer size

3. **Cold Start Latency**
   - Metric: `orderbook.coldstart.duration`
   - Visualization: Histogram/Heatmap
   - Percentiles: p50, p95, p99

4. **Stale Update Rate**
   - Metric: `orderbook.update.stale`
   - Visualization: Stat panel with rate
   - Threshold: Warning at 50/hour

5. **Recovery Success Rate**
   - Metrics: `orderbook.updates.replayed` (valid vs buffered)
   - Visualization: Gauge
   - Formula: valid/buffered * 100

---

## Alert Rules

### Critical Alerts

```yaml
# High Gap Detection Rate
- alert: OrderbookHighGapRate
  expr: rate(orderbook_gap_detected_total[5m]) > 0.05  # 3 gaps/minute
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "High orderbook gap rate for {{ $labels.symbol }}"
    description: "{{ $labels.symbol }} has {{ $value }} gaps/sec indicating network issues"

# Stuck Cold Start
- alert: OrderbookStuckColdStart
  expr: orderbook_buffer_size > 500
  for: 30s
  labels:
    severity: critical
  annotations:
    summary: "Orderbook stuck in cold start for {{ $labels.symbol }}"
    description: "{{ $labels.symbol }} has {{ $value }} buffered updates, snapshot may be failing"
```

### Warning Alerts

```yaml
# Elevated Stale Updates
- alert: OrderbookHighStaleRate
  expr: rate(orderbook_update_stale_total[5m]) > 0.01  # 0.6 stale/minute
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "High stale update rate for {{ $labels.symbol }}"
    description: "{{ $labels.symbol }} has {{ $value }} stale updates/sec"

# Slow Cold Start
- alert: OrderbookSlowColdStart
  expr: histogram_quantile(0.95, rate(orderbook_coldstart_duration_bucket[5m])) > 2000
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Slow orderbook cold start (p95: {{ $value }}ms)"
    description: "Orderbook cold start latency is elevated, check snapshot API"
```

---

## Troubleshooting Guide

### High Gap Detection Rate

**Symptoms**: `orderbook.gap.detected` increasing rapidly

**Possible Causes**:
1. Network packet loss
2. Provider WebSocket instability
3. Rate limiting issues
4. Overloaded system (CPU/memory)

**Investigation**:
```promql
# Check gap sizes
max by (symbol) (orderbook_gap_detected{gap_size})

# Correlate with network errors
rate(provider_errors_total[5m])
```

**Resolution**:
- Check network connectivity
- Verify WebSocket connection health
- Review provider logs for errors
- Scale up resources if CPU/memory constrained

---

### Large Buffer Sizes

**Symptoms**: `orderbook.buffer.size` > 100

**Possible Causes**:
1. Snapshot REST API delays
2. Snapshot API rate limiting
3. Network issues fetching snapshots

**Investigation**:
```promql
# Check snapshot application rate
rate(orderbook_snapshot_applied_total[5m])

# Check cold start duration
orderbook_coldstart_duration
```

**Resolution**:
- Verify REST API health
- Check rate limit headroom
- Increase `book_refresh_interval` if hitting limits
- Review REST client logs

---

### High Stale Update Rate

**Symptoms**: `orderbook.update.stale` increasing

**Possible Causes**:
1. Out-of-order message delivery
2. Clock skew between provider and system
3. Replay logic issues

**Investigation**:
```promql
# Check sequence patterns
orderbook_update_stale{first_seq,final_seq,current_seq}
```

**Resolution**:
- Usually benign if < 1% of total updates
- If persistent, check provider message ordering
- Verify system time synchronization (NTP)

---

## Integration Example

### Grafana Dashboard JSON

See `docs/grafana/orderbook-telemetry-dashboard.json` (to be created)

### Prometheus Recording Rules

```yaml
groups:
  - name: orderbook_derived
    interval: 30s
    rules:
      - record: orderbook:gap_rate:5m
        expr: rate(orderbook_gap_detected_total[5m])
      
      - record: orderbook:stale_rate:5m
        expr: rate(orderbook_update_stale_total[5m])
      
      - record: orderbook:snapshot_rate:5m
        expr: rate(orderbook_snapshot_applied_total[5m])
      
      - record: orderbook:coldstart:p95
        expr: histogram_quantile(0.95, rate(orderbook_coldstart_duration_bucket[5m]))
```

---

## References

- **Implementation**: `internal/adapters/binance/book_assembler.go`
- **Provider Integration**: `internal/adapters/binance/provider.go`
- **Architecture**: `docs/PROVIDER_AGNOSTIC_ORDERBOOK.md`
- **Buffering**: `docs/BUFFERING_IMPLEMENTATION.md`
