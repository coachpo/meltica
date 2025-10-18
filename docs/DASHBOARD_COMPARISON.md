# Dashboard Structure Comparison

## Current State (3 Dashboards)

### 1. Pool Dashboard
**Focus**: Object pool utilization  
**Metrics**: 5 pool metrics  
**Strengths**: Good pool visibility  
**Gaps**: Isolated view, no context of why pools are stressed

### 2. Provider/Telemetry Dashboard  
**Focus**: Dispatcher + Provider + Databus metrics  
**Metrics**: 13+ metrics mixed together  
**Strengths**: Comprehensive metric coverage  
**Gaps**: No clear hierarchy, hard to diagnose issues, mixes concerns

### 3. Orderbook Dashboard
**Focus**: Book assembly quality  
**Metrics**: 6 orderbook metrics  
**Strengths**: Good orderbook-specific signals  
**Gaps**: No quality score, no architectural context

---

## Proposed State (5 Dashboards)

### 1. 🆕 System Overview (NEW)
**Purpose**: Single pane of glass for overall health  
**Audience**: Operations team, executives, first responder  
**Key Insights**:
- Is the system healthy? (6 traffic-light indicators)
- Where is the bottleneck? (Event flow funnel)
- Which component needs attention? (Component health grid)

**Usage Pattern**: Start here every time

---

### 2. ♻️ Event Pipeline (ENHANCED from Provider)
**Purpose**: Deep dive into data flow  
**Audience**: Engineers debugging event loss or latency  
**Key Insights**:
- Where are events being dropped? (By reason)
- What's the end-to-end latency? (Trace-based)
- Is the dispatcher keeping up? (Buffer sizes, processing latency)
- Are consumers receiving events? (Fanout sizes, delivery errors)

**Usage Pattern**: Drill down from System Overview when error rate spikes

---

### 3. ♻️ Orderbook Quality (REFINED)
**Purpose**: Book assembly integrity monitoring  
**Audience**: Trading desk, risk management  
**Key Insights**:
- Which symbols are unreliable? (Quality score table)
- Are we recovering from gaps? (Buffer buildup patterns)
- How fast do we cold start? (Latency distribution)

**Additions**:
- Quality score formula (combines gap rate + stale rate)
- Recovery pattern visualization (gap → snapshot → ready)
- Top 5 problem symbols table

**Usage Pattern**: Verify orderbook integrity before trading decisions

---

### 4. ♻️ Pool Health (MINOR ENHANCEMENTS)
**Purpose**: Memory pool utilization  
**Audience**: Engineers investigating memory pressure  
**Key Insights**:
- Are we running out of pooled objects? (Utilization)
- Are borrows becoming slow? (Latency distribution)
- Are there leaks? (Monotonic active count increases)

**Additions**:
- Pool contention indicator (p99 borrow latency > 5ms)
- Leak detection visual (active objects trend)

**Usage Pattern**: Investigate when Pool Pressure alert fires

---

### 5. 🆕 SLI/SLO (NEW)
**Purpose**: Production readiness & compliance  
**Audience**: SRE team, management  
**Key Insights**:
- Are we meeting availability SLO? (99.9% target)
- Are we within latency budget? (p99 < 10ms)
- How fast are we burning error budget? (Burn rate)

**SLI Definitions**:
| SLI | Target | Current | Status |
|-----|--------|---------|--------|
| Availability | 99.9% | 99.95% | ✅ |
| Latency p99 | < 10ms | 8.2ms | ✅ |
| Error Rate | < 0.1% | 0.05% | ✅ |
| Book Quality | > 99% | 99.8% | ✅ |
| Pool Latency | < 1ms p99 | 0.3ms | ✅ |

**Usage Pattern**: Weekly SLO review meetings, on-call handoffs

---

## Workflow Examples

### Scenario 1: "System is slow"
**Current**: Check all 3 dashboards manually, correlate timestamps  
**Proposed**:
1. **System Overview** → See "Dispatcher Health" is red (p99 > 50ms)
2. Click link → **Event Pipeline** → See processing latency spike for BTC-USDT
3. Click exemplar → **Jaeger trace** → See slow pool borrow (15ms)
4. Click link → **Pool Health** → See Event pool at 98% utilization
5. **Root cause**: Need to increase Event pool capacity

### Scenario 2: "Events are being dropped"
**Current**: Provider dashboard → Look for drop counters, unclear why  
**Proposed**:
1. **System Overview** → See "Error Rate" is red
2. **Event Flow Funnel** → See "Dropped (leakage)" spike
3. Click link → **Event Pipeline** → Filter by `reason` label
4. See `reason="buffer_full"` for BTC-USDT
5. **Root cause**: Dispatcher buffer full for BTC-USDT (hot symbol)

### Scenario 3: "Can we trust orderbook for symbol X?"
**Current**: Orderbook dashboard → Look at gaps/stale rates manually  
**Proposed**:
1. **Orderbook Quality** → Check quality score table
2. Filter by `symbol="X"` → Score = 0.92 (yellow, < 0.95)
3. See gap rate = 0.05/s (elevated)
4. Check recovery pattern → See frequent gap → snapshot cycles
5. **Recommendation**: Reduce trading size for symbol X

### Scenario 4: "Are we production ready?"
**Current**: No clear answer  
**Proposed**:
1. **SLI/SLO Dashboard** → See all SLIs green except Book Quality (yellow)
2. Check error budget burn rate → 10% consumed (safe)
3. Review 30-day trend → 99.95% availability (above target)
4. **Answer**: Yes, production ready with caveat on orderbook for low-liquidity symbols

---

## Dashboard Navigation Flow

```
System Overview (START HERE)
    │
    ├──[High Error Rate]──→ Event Pipeline ──→ Pool Health
    │                                      └──→ Orderbook Quality
    │
    ├──[Pool Pressure]────→ Pool Health
    │
    ├──[Low Quality]──────→ Orderbook Quality
    │
    └──[SLO Compliance]───→ SLI/SLO Dashboard
```

---

## Metric Coverage Map

| Metric | System Overview | Event Pipeline | Orderbook | Pool | SLI/SLO |
|--------|----------------|----------------|-----------|------|---------|
| `dispatcher.events.ingested` | ✅ | ✅ | - | - | ✅ |
| `dispatcher.events.dropped` | ✅ | ✅ | - | - | ✅ |
| `dispatcher.events.duplicate` | - | ✅ | - | - | - |
| `dispatcher.events.buffered` | - | ✅ | - | - | - |
| `dispatcher.processing.duration` | ✅ | ✅ | - | - | ✅ |
| `dispatcher.routing.version` | - | ✅ | - | - | - |
| `databus.events.published` | ✅ | ✅ | - | - | - |
| `databus.subscribers` | - | ✅ | - | - | - |
| `databus.delivery.errors` | ✅ | ✅ | - | - | ✅ |
| `databus.fanout.size` | - | ✅ | - | - | - |
| `pool.objects.borrowed` | - | - | - | ✅ | - |
| `pool.objects.active` | ✅ | - | - | ✅ | - |
| `pool.borrow.duration` | - | - | - | ✅ | ✅ |
| `pool.capacity` | ✅ | - | - | ✅ | - |
| `pool.available` | - | - | - | ✅ | - |
| `orderbook.gap.detected` | ✅ | - | ✅ | - | ✅ |
| `orderbook.buffer.size` | - | - | ✅ | - | - |
| `orderbook.update.stale` | ✅ | - | ✅ | - | - |
| `orderbook.snapshot.applied` | - | - | ✅ | - | - |
| `orderbook.coldstart.duration` | - | - | ✅ | - | - |
| `provider.reconnections` | ✅ | ✅ | - | - | - |

**Coverage**: All 23 metrics used, no redundancy

---

## Alerting Integration

Each dashboard links to relevant alerts:

### System Overview
- `HighEventDropRate` → Links to Event Pipeline filtered by drop reason
- `PoolExhaustion` → Links to Pool Health filtered by exhausted pool
- `OrderbookGapStorm` → Links to Orderbook Quality filtered by symbol

### Event Pipeline
- `SlowDispatcherProcessing` → Shows latency heatmap with exemplar links
- `HighBufferedEvents` → Shows buffer sizes by event type

### Orderbook Quality
- `FrequentOrderbookGaps` → Shows gap rate trend by symbol
- `SlowColdStart` → Shows cold start duration distribution

### Pool Health
- `PoolExhaustion` → Shows utilization trend
- `SlowPoolBorrow` → Shows borrow latency distribution

### SLI/SLO
- `SLOViolation` → Shows which SLI is out of compliance
- `HighErrorBudgetBurn` → Shows burn rate trend

---

## Implementation Priority

### Phase 1: Immediate Value (1-2 hours)
✅ Create System Overview dashboard  
✅ Add quality score to Orderbook dashboard  
✅ Rename "Provider Health" → "Event Pipeline"

### Phase 2: Enhanced Visibility (2-4 hours)
🔧 Add event flow funnel to System Overview  
🔧 Add component health traffic lights  
🔧 Add drill-down links between dashboards

### Phase 3: Production Readiness (1 day)
📊 Create SLI/SLO dashboard  
📊 Configure alerting rules  
📊 Add anomaly detection

### Phase 4: Advanced (1-2 days)
🚀 Instrument Control Bus metrics  
🚀 Add trace-based E2E latency  
🚀 Add consumer acknowledgment tracking

---

## Success Metrics

How we'll know the new dashboards are better:

1. **Time to detect issues**: < 30 seconds (from System Overview)
2. **Time to diagnose root cause**: < 5 minutes (via drill-down links)
3. **Dashboard switches per investigation**: < 3 (down from 5+)
4. **False positive alerts**: < 10% (via SLI/SLO thresholds)
5. **Operator satisfaction**: > 4/5 stars (survey after 2 weeks)

---

## Migration Plan

### Week 1: Beta Testing
- Deploy new dashboards alongside existing
- Gather feedback from 2-3 engineers
- Iterate on layout and thresholds

### Week 2: Rollout
- Set System Overview as default landing page
- Update documentation and runbooks
- Train operations team

### Week 3: Deprecation
- Archive old dashboards (keep for reference)
- Update all documentation links
- Collect final feedback

### Week 4: Optimization
- Tune alert thresholds based on false positives
- Add requested enhancements
- Document lessons learned

---

## Summary

| Aspect | Current | Proposed | Improvement |
|--------|---------|----------|-------------|
| **Dashboards** | 3 | 5 | More targeted |
| **First Glance Health** | No | Yes (System Overview) | ✅ 10x faster |
| **Drill-Down Workflow** | Manual | Linked | ✅ 3x faster |
| **Architecture Alignment** | Low | High | ✅ Better understanding |
| **SLO Tracking** | No | Yes | ✅ Production ready |
| **Metric Coverage** | 23/23 | 23/23 | Same |
| **Redundancy** | Some | None | ✅ Cleaner |

**Key Benefit**: Observability aligned with how the system is **built** (architecture) and how teams **operate** it (workflows).
