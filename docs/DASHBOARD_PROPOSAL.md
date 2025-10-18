# Meltica Dashboard Proposal

**Objective**: Redesign Grafana dashboards to align with system architecture, event lifecycle, and observability best practices.

---

## Current State Analysis

### Existing Dashboards
1. **Pool Dashboard** - Focuses on object pool metrics (5 pools)
2. **Provider/Telemetry Dashboard** - Dispatcher + Provider + Databus metrics
3. **Orderbook Dashboard** - Book assembly quality signals

### Issues Identified
- **Component-centric**, not **flow-centric** - Hard to trace events end-to-end
- **No unified health overview** - Requires switching between 3 dashboards
- **Missing architectural context** - Doesn't reflect Provider → Dispatcher → Databus → Consumer flow
- **No SLI/SLO tracking** - Can't assess if system is meeting performance targets
- **Limited troubleshooting workflows** - No drill-down paths for common issues

---

## Architecture-Aligned Dashboard Strategy

### Data Flow Architecture
```
Provider (Binance)
  ├─ WS Client (ingestion)
  ├─ Book Assembler (integrity)
  └─ Parser (normalization)
         ↓
Dispatcher (ordering + routing + dedup)
         ↓
Data Bus (pub/sub fanout)
         ↓
Consumers (strategy lambdas)

Control Bus ←→ Dispatcher (routing updates)
Pool Manager ←→ All Components (bounded pools)
```

### Proposed Dashboard Structure

## 1. **System Overview Dashboard** (NEW)
**Purpose**: Single pane of glass for overall system health

**Layout**:

### Row 1: System Health Overview (Stats)
- **Ingestion Health**: Events/sec ingested by Dispatcher
- **Delivery Health**: Events/sec delivered via Databus  
- **Error Rate**: Drops + Delivery errors combined
- **Pool Pressure**: Highest pool utilization across all pools
- **Orderbook Quality**: Gap rate + stale rate combined score

### Row 2: Event Flow Funnel (Graph)
Single visualization showing end-to-end throughput:
- `dispatcher.events.ingested` (top of funnel)
- `databus.events.published` (middle)
- `dispatcher.events.dropped` (leakage - stacked as negative)
- `databus.delivery.errors` (leakage - stacked as negative)

**Purpose**: Immediately shows if events are being lost in the pipeline

### Row 3: Component Health Status (Stats with Traffic Lights)
- **Provider Health**: WS reconnection rate (green < 0.01/min, yellow < 0.1/min, red ≥ 0.1/min)
- **Dispatcher Health**: Processing p99 latency (green < 10ms, yellow < 50ms, red ≥ 50ms)
- **Databus Health**: Delivery error rate (green < 0.1%, yellow < 1%, red ≥ 1%)
- **Pool Health**: Max utilization across pools (green < 80%, yellow < 95%, red ≥ 95%)
- **Orderbook Health**: Gap detection rate (green < 0.01/s, yellow < 0.1/s, red ≥ 0.1/s)

### Row 4: Event Types Breakdown (Time Series)
Stacked area chart showing ingestion by `event_type`:
- BookSnapshot
- BookUpdate
- Trade
- Ticker
- Kline
- ExecReport

### Row 5: Quick Links & Alerts
- Links to detailed dashboards
- Active alert summary (requires alerting rules)

**Templating Variables**:
- `environment` (dev/staging/prod)
- `provider` (binance/fake)
- Time range selector

---

## 2. **Event Pipeline Dashboard** (ENHANCED)
**Purpose**: Deep dive into event flow from Provider → Dispatcher → Databus → Consumers

**Layout**:

### Row 1: Provider Ingestion
- **Ingestion Rate** (by symbol, event_type) - Time series
- **Provider Reconnections** - Stat + time series
- **Raw Events Received** - Counter

### Row 2: Dispatcher Processing
- **Processing Latency** - Heatmap (p50/p95/p99)
- **Buffered Events** - Time series by event_type
- **Duplicate Events** - Time series (should be near 0)
- **Dropped Events** - Time series by reason (buffer_full, invalid, etc.)
- **Routing Version** - Gauge (shows routing table changes)

### Row 3: Databus Delivery
- **Published Events** - Time series by event_type
- **Fanout Size** - Histogram (shows subscriber distribution)
- **Active Subscribers** - Time series by event_type
- **Delivery Errors** - Time series by error_type

### Row 4: End-to-End Latency (CRITICAL)
**New Metric Recommendation**: Add trace-based E2E latency
- Time from Provider ingestion to Databus delivery
- Requires span correlation between `dispatcher.process_event` and `databus.Publish`

### Row 5: Event Pipeline Health Table
Table showing per-symbol health:
| Symbol | Ingest Rate | Drop Rate | Duplicate Rate | Delivery Rate | P99 Latency |
|--------|-------------|-----------|----------------|---------------|-------------|

**Templating Variables**:
- `environment`, `provider`, `event_type`, `symbol`

---

## 3. **Orderbook Quality Dashboard** (REFINED)
**Purpose**: Orderbook assembly integrity and cold start performance

**Current panels are good, but suggest these additions**:

### Row 1: Quality Scores (NEW)
- **Overall Quality Score**: Formula combining gap rate, stale rate, snapshot frequency
  ```
  quality = 1.0 - (gap_rate * 10 + stale_rate * 5) / max_threshold
  ```
- **Symbols Needing Attention**: Table showing worst 5 symbols by quality score

### Row 2: Current Metrics (Keep existing)
- Gap Rate
- Stale Update Rate
- Snapshots Applied
- Buffered Updates

### Row 3: Cold Start Performance (Keep existing)
- Cold Start Duration (p50/p95/p99)
- Cold Start Frequency

### Row 4: Recovery Patterns (NEW)
- **Gap Detection Events** - Time series (spikes indicate network issues)
- **Buffer Buildup** - Time series showing `orderbook.buffer.size` (high = recovery mode)
- **Recovery Duration** - Time from gap detection to ready state

### Row 5: Per-Symbol Detail Table (Keep existing)
| Symbol | Gap Rate | Stale Rate | Buffer Size | Last Snapshot | Quality Score |
|--------|----------|------------|-------------|---------------|---------------|

**Templating Variables**:
- `environment`, `provider`, `symbol`

---

## 4. **Pool Health Dashboard** (KEEP + MINOR ENHANCEMENTS)
**Purpose**: Memory pool utilization and borrow latency

**Current panels are excellent**, suggest minor additions:

### Add to existing:
- **Pool Contention Indicator**: Shows when `pool.borrow.duration` p99 > 5ms
- **Leak Detection Alert**: Shows pools where `active` is monotonically increasing
- **Pool Efficiency**: Formula showing `borrowed / (capacity * uptime_seconds)`

---

## 5. **SLI/SLO Dashboard** (NEW)
**Purpose**: Track Service Level Objectives for production readiness

### Critical SLIs:

| SLI | Target | Query |
|-----|--------|-------|
| **Availability** | 99.9% uptime | 1 - (sum(rate(dispatcher.events.dropped[5m])) / sum(rate(dispatcher.events.ingested[5m]))) |
| **Latency** | p99 < 10ms | histogram_quantile(0.99, rate(dispatcher.processing.duration_bucket[5m])) < 0.010 |
| **Error Rate** | < 0.1% | sum(rate(databus.delivery.errors[5m])) / sum(rate(databus.events.published[5m])) < 0.001 |
| **Orderbook Quality** | > 99% | 1 - sum(rate(orderbook.gap.detected[5m])) / sum(rate(orderbook.snapshot.applied[5m])) > 0.99 |
| **Pool Availability** | < 1ms p99 borrow latency | histogram_quantile(0.99, rate(pool.borrow.duration_bucket[5m])) < 0.001 |

**Layout**:
- **SLO Compliance Grid**: Traffic light indicators for each SLI
- **Error Budget Burn Rate**: Shows how fast error budget is being consumed
- **SLO History**: 30-day compliance trend

---

## Implementation Recommendations

### Phase 1: Quick Wins (1-2 hours)
1. ✅ Create **System Overview Dashboard** using existing metrics
2. ✅ Enhance **Orderbook Dashboard** with quality score formula
3. ✅ Rename "Provider Health" → "Event Pipeline Dashboard"
4. ✅ Add event flow funnel visualization

### Phase 2: Metric Enhancements (4-6 hours)
1. 🔧 Add trace-based E2E latency measurement (span correlation)
2. 🔧 Add `controlbus.*` metrics (currently not instrumented per TELEMETRY_POINTS.md)
3. 🔧 Add consumer delivery acknowledgment metric (delivery → processing complete)

### Phase 3: Advanced (1-2 days)
1. 📊 Create SLI/SLO Dashboard with alerting rules
2. 📊 Add anomaly detection for latency spikes
3. 📊 Create drill-down links between dashboards (e.g., click "high drop rate" → jumps to Event Pipeline filtered by reason)

---

## Best Practices Applied

### 1. **USE Method** (Utilization, Saturation, Errors)
- **Utilization**: Pool utilization, buffer fill levels
- **Saturation**: Borrow duration spikes, buffer full drops
- **Errors**: Drops, delivery errors, gaps, stale updates

### 2. **RED Method** (Rate, Errors, Duration)
- **Rate**: Ingestion rate, publish rate
- **Errors**: Drop rate, delivery error rate
- **Duration**: Processing latency, borrow latency, cold start duration

### 3. **Four Golden Signals** (Google SRE)
- **Latency**: Dispatcher processing, pool borrow, cold start
- **Traffic**: Events/sec ingested and delivered
- **Errors**: Drops + delivery errors + gaps
- **Saturation**: Pool utilization, buffer sizes

### 4. **Data Ink Ratio**
- Remove clutter: Already done (label exclusions ✅)
- Focus on insights: Quality scores, health indicators
- Use color sparingly: Traffic lights only for thresholds

### 5. **Troubleshooting Workflows**
Dashboard should answer:
1. **Is the system healthy?** → System Overview
2. **Where is the bottleneck?** → Event Pipeline (funnel visualization)
3. **Are orderbooks reliable?** → Orderbook Quality
4. **Are we running out of memory?** → Pool Health
5. **Are we meeting SLOs?** → SLI/SLO Dashboard

---

## Grafana Best Practices

### Layout Guidelines
- **Stats at top**: Immediate health signals
- **Graphs below**: Detailed time series for investigation
- **Tables at bottom**: Drill-down detail
- **Max 24 columns**: Consistent grid layout
- **Row headers**: Logical grouping with collapsible rows

### Visualization Selection
- **Stat panels**: Single value with threshold colors (health indicators)
- **Time series**: Trends over time (rates, latencies)
- **Heatmaps**: Distribution analysis (latency percentiles)
- **Bar gauge**: Relative comparisons (per-symbol rates)
- **Table**: Detailed breakdowns (summary tables)

### Templating Strategy
- **Cascading variables**: `environment` → `provider` → `symbol`
- **Multi-select**: Allow "All" for high-level view
- **Query-based**: Dynamic lists from metrics
- **URL sync**: Enable shareable filtered views

### Performance Optimizations
- **Appropriate intervals**: Use `$__rate_interval` not fixed `[1m]`
- **Limit cardinality**: Filter by `environment` to reduce query load
- **Panel queries**: < 10 series per panel for fast rendering
- **Refresh rate**: 30s default, 5s for critical panels only

---

## Migration Path

### Immediate (Keep existing dashboards as-is)
Existing dashboards remain functional for users familiar with them.

### New Dashboards (Create alongside)
1. `meltica-system-overview` (NEW)
2. `meltica-event-pipeline` (ENHANCED from provider-health)
3. `meltica-orderbook-quality` (REFINED from orderbook)
4. `meltica-pool-health` (MINOR ENHANCEMENTS)
5. `meltica-sli-slo` (NEW)

### Deprecation Timeline
- **Week 1-2**: New dashboards in beta, gather feedback
- **Week 3-4**: Set new dashboards as defaults
- **Week 5+**: Archive old dashboards (keep for reference)

---

## Alerting Rules (Future Work)

Critical alerts to implement after dashboards:

```yaml
# Example Prometheus alerting rules
groups:
  - name: meltica_critical
    rules:
      - alert: HighEventDropRate
        expr: rate(meltica_dispatcher_events_dropped[5m]) > 10
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High event drop rate: {{ $value }} events/sec"

      - alert: PoolExhaustion
        expr: |
          meltica_pool_objects_active / meltica_pool_capacity > 0.95
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Pool {{ $labels.pool_name }} is 95%+ utilized"

      - alert: OrderbookGapStorm
        expr: rate(meltica_orderbook_gap_detected[1m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Frequent orderbook gaps for {{ $labels.symbol }}"
```

---

## Summary

### Key Improvements
1. ✅ **Architecture-aligned**: Dashboards mirror Provider → Dispatcher → Databus flow
2. ✅ **Single pane of glass**: System Overview for holistic health
3. ✅ **Troubleshooting workflows**: Clear drill-down paths
4. ✅ **SLI/SLO tracking**: Production-ready observability
5. ✅ **Best practices**: USE, RED, Four Golden Signals applied

### Next Steps
1. **Review & approve** this proposal
2. **Create System Overview dashboard** (Phase 1)
3. **Refine existing dashboards** with proposed enhancements
4. **Instrument missing metrics** (Control Bus, E2E latency)
5. **Deploy SLI/SLO dashboard** for production readiness
