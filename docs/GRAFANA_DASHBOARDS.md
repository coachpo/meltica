# Meltica Grafana Dashboards Guide

This document provides an overview of all available Grafana dashboards for monitoring the Meltica streaming gateway.

## Dashboard Index

1. **System Overview** - High-level system health and metrics across all components
2. **Event Pipeline** - Event ingestion, processing, and delivery metrics
3. **Orderbook** - Orderbook assembly, gaps, and synchronization metrics
4. **Pool** - Memory pool utilization and performance metrics

---

## 1. System Overview Dashboard

**File:** `grafana-overview-dashboard.json`  
**UID:** `meltica-system-overview`

### Purpose
Provides a high-level view of the entire system health, combining key metrics from all subsystems into a single dashboard. Ideal for operations teams and incident response.

### Key Metrics
- **Event Throughput**: Total events/sec ingested by the system
- **Pool Utilization**: Average memory pool utilization across all pools
- **Orderbook Gaps**: Rate of sequence gaps detected
- **Events Dropped**: Rate of events dropped due to errors
- **Delivery Errors**: Rate of event delivery failures
- **Active Subscribers**: Number of active event subscribers

### Sections
1. **System Status** - Top-level KPIs
2. **Event Pipeline** - Event flow visualization
3. **Memory Pools** - Pool utilization and borrow rates
4. **Orderbook Health** - Snapshot and gap rates
5. **Performance** - Latency metrics and health score

### When to Use
- Daily operations monitoring
- Incident detection and triage
- Executive dashboards
- First stop for troubleshooting

### Variables
- `environment`: Filter by environment (development, production, etc.)

---

## 2. Event Pipeline Dashboard

**File:** `grafana-pipeline-dashboard.json`  
**UID:** `meltica-pipeline-metrics`

### Purpose
Deep dive into event processing pipeline, tracking events from ingestion through dispatcher to databus delivery. Focuses on event flow, performance, and error rates.

### Metrics Breakdown

#### Dispatcher Metrics (`dispatcher.*`)
- `dispatcher.events.ingested` - Events received from providers
- `dispatcher.events.dropped` - Events dropped (with reason labels)
- `dispatcher.events.duplicate` - Duplicate events detected
- `dispatcher.events.buffered` - Events currently buffered for ordering
- `dispatcher.processing.duration` - Event processing time histogram

#### Databus Metrics (`databus.*`)
- `databus.events.published` - Events published to subscribers
- `databus.subscribers` - Active subscriber count per event type
- `databus.delivery.errors` - Event delivery failures (with error labels)
- `databus.fanout.size` - Subscriber count per fanout operation

### Key Panels
1. **Event Flow** - Ingested vs Published vs Dropped timeline
2. **Events by Provider** - Breakdown by data source (binance, fake, etc.)
3. **Events by Type** - Distribution across event types (book_snapshot, trade, etc.)
4. **Processing Duration** - p50/p95/p99 latency
5. **Delivery Errors** - Error rates by type and reason
6. **Pipeline Health Summary** - Table view with all metrics

### Labels Used
- `environment`: deployment environment
- `event_type`: type of event (book_snapshot, book_delta, trade)
- `provider`: data provider (binance, fake)
- `reason`: drop reason for dropped events
- `error`: error type for delivery failures

### When to Use
- Investigating event loss or delays
- Analyzing provider performance
- Troubleshooting delivery issues
- Capacity planning for event volume

### Variables
- `environment`: Filter by environment
- `event_type`: Filter by event type
- `provider`: Filter by provider

---

## 3. Orderbook Dashboard

**File:** `grafana-orderbook-dashboard.json`  
**UID:** `meltica-orderbook-metrics`

### Purpose
Monitor orderbook assembly health, focusing on sequence integrity, gap detection, and cold start performance. Critical for ensuring accurate orderbook state.

### Metrics Breakdown

#### Orderbook Metrics (`orderbook.*`)
- `orderbook.gap.detected` - Sequence gaps requiring restart
  - Labels: `symbol`, `gap_size`, `recovery_action`
- `orderbook.buffer.size` - Updates buffered before snapshot
  - Labels: `symbol`
- `orderbook.update.stale` - Stale updates rejected
  - Labels: `symbol`
- `orderbook.snapshot.applied` - Snapshots applied successfully
  - Labels: `symbol`
- `orderbook.updates.replayed` - Buffered updates replayed
  - Labels: `symbol`, `buffered`, `valid`
- `orderbook.coldstart.duration` - Time to first ready state (histogram)
  - Labels: `symbol`

### Key Panels
1. **Snapshot Applied Rate** - Frequency of snapshot refreshes
2. **Gap Detection Events** - Sequence integrity issues
3. **Cold Start Duration** - p50/p95/p99 initialization time
4. **Buffer Size** - Pre-snapshot update buffering
5. **Updates Replayed** - Buffered update replay after snapshot
6. **Stale Updates** - Out-of-order update rejection rate
7. **Orderbook Health Summary** - Per-symbol health table

### Alert Conditions
- **Gap Rate > 0.1 ops/s**: High gap detection indicates connectivity or sequence issues
- **Buffer Size > 50**: Prolonged buffering suggests snapshot delays
- **Stale Rate > 10 ops/s**: High stale rate indicates clock skew or ordering issues

### When to Use
- Investigating orderbook accuracy issues
- Monitoring gap detection and recovery
- Analyzing cold start performance
- Troubleshooting sequence synchronization

### Variables
- `environment`: Filter by environment
- `symbol`: Filter by trading pair (BTCUSDT, ETHUSDT, etc.)

---

## 4. Pool Dashboard

**File:** `grafana-pool-dashboard.json`  
**UID:** `meltica-pool-metrics`

### Purpose
Monitor memory pool efficiency, object reuse, and resource utilization. Critical for understanding memory allocation patterns and preventing pool exhaustion.

### Metrics Breakdown

#### Pool Metrics (`pool.*`)
- `pool.objects.borrowed` - Objects borrowed from pool (counter)
- `pool.objects.returned` - Objects returned to pool (counter)
- `pool.objects.active` - Objects currently in use (gauge)
- `pool.capacity` - Maximum pool capacity (gauge)
- `pool.available` - Objects available for borrowing (gauge)
- `pool.utilization` - Calculated utilization (active/capacity)
- `pool.borrow.duration` - Time to borrow object (histogram)

### Pool Types
- **Event** (`*schema.Event`) - Canonical event objects
- **OrderRequest** (`*schema.OrderRequest`) - Order request objects
- **ParseFrame** (`*schema.ParseFrame`) - Frame parsing objects
- **WsFrame** (`*schema.WsFrame`) - WebSocket frame objects

### Key Panels
1. **Pool Utilization by Type** - Bar gauge showing all pools
2. **Active Objects Over Time** - Usage trends with capacity lines
3. **Borrow & Return Rates** - Pool activity rates
4. **Borrow Latency** - p50/p95/p99 borrow times
5. **Available Objects** - Pool availability monitoring
6. **Pool Details** - Comprehensive table view

### Alert Conditions
- **Pool Utilization > 90%**: Pool nearing capacity
- **Available Objects == 0**: Pool exhausted (critical)
- **Borrow Latency p99 > 100ms**: Contention or allocation issues

### When to Use
- Capacity planning and sizing
- Investigating memory allocation issues
- Detecting memory leaks (active not decreasing)
- Optimizing pool configuration

### Variables
- `environment`: Filter by environment
- `pool_name`: Filter by pool name
- `object_type`: Filter by object type

---

## Common Metric Labels

All metrics include these standard labels:

- `environment`: Deployment environment (development, production, staging)
- `service`: Always "meltica"
- `instance`: OpenTelemetry collector instance
- `job`: Prometheus job name
- `exported_job`: Original service name

---

## Import Instructions

### Grafana Cloud or Self-Hosted

1. **Navigate to Dashboards**
   - Go to Grafana → Dashboards → New → Import

2. **Upload JSON**
   - Click "Upload JSON file"
   - Select one of the dashboard JSON files
   - Or paste the JSON content directly

3. **Configure Data Source**
   - In the import dialog, you'll see a "Datasource" dropdown
   - Select your Prometheus data source
   - Click "Import"

4. **After Import - Configure Variables**
   - The dashboard will open after import
   - At the top, you'll see variable dropdowns (Datasource, Environment, etc.)
   - **Important**: Select your actual Prometheus datasource from the "Datasource" dropdown
   - The other variables (Environment, Pool Name, etc.) should auto-populate after selecting the datasource
   - If variables show "No options found", see Troubleshooting section below

5. **Recommended Import Order**
   1. System Overview (first - shows if metrics are flowing)
   2. Event Pipeline
   3. Orderbook
   4. Pool

---

## Prerequisites

### Required Setup

1. **OpenTelemetry Collector**
   - Metrics endpoint exposed on port 8889
   - OTLP receiver configured
   - Prometheus exporter enabled

2. **Prometheus**
   - Scraping OpenTelemetry Collector metrics endpoint
   - Retention period: minimum 7 days recommended

3. **Gateway Configuration**
   - `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable set
   - Telemetry enabled (default)
   - Resource attributes configured (environment, service.name)

### Verification

Check metrics are available:
```bash
# Query Prometheus for meltica metrics
curl 'http://prometheus:9090/api/v1/label/__name__/values' | grep -E 'pool|dispatcher|databus|orderbook'

# Expected output includes:
# - pool_objects_active
# - dispatcher_events_ingested
# - databus_events_published
# - orderbook_gap_detected
```

---

## Metric Naming Convention

### OpenTelemetry to Prometheus Conversion

OpenTelemetry metrics use dots (`.`), but Prometheus converts them to underscores (`_`):

| OpenTelemetry | Prometheus |
|---------------|------------|
| `pool.objects.borrowed` | `pool_objects_borrowed` |
| `dispatcher.events.ingested` | `dispatcher_events_ingested` |
| `orderbook.gap.detected` | `orderbook_gap_detected` |

### Histogram Metrics

Histograms create multiple series:
- `<metric>_bucket{le="..."}` - Cumulative buckets
- `<metric>_sum` - Sum of all observed values
- `<metric>_count` - Count of observations

Example for `pool.borrow.duration`:
```
pool_borrow_duration_bucket{le="0.1"}
pool_borrow_duration_bucket{le="0.5"}
pool_borrow_duration_sum
pool_borrow_duration_count
```

---

## Alerting Recommendations

### Critical Alerts

1. **Pool Exhaustion**
   ```promql
   pool_available{environment="production"} == 0
   ```
   - Severity: Critical
   - Duration: 1 minute
   - Action: Scale up capacity or investigate leaks

2. **High Drop Rate**
   ```promql
   rate(dispatcher_events_dropped{environment="production"}[5m]) > 1
   ```
   - Severity: Critical
   - Duration: 5 minutes
   - Action: Check ordering buffer capacity

3. **Orderbook Gap Storm**
   ```promql
   rate(orderbook_gap_detected{environment="production"}[1m]) > 0.1
   ```
   - Severity: Critical
   - Duration: 2 minutes
   - Action: Check exchange connectivity

### Warning Alerts

4. **High Pool Utilization**
   ```promql
   pool_utilization{environment="production"} > 0.9
   ```
   - Severity: Warning
   - Duration: 5 minutes
   - Action: Plan capacity increase

5. **Delivery Errors**
   ```promql
   rate(databus_delivery_errors{environment="production"}[5m]) > 0.1
   ```
   - Severity: Warning
   - Duration: 5 minutes
   - Action: Check subscriber health

6. **High Processing Latency**
   ```promql
   histogram_quantile(0.99, rate(dispatcher_processing_duration_bucket{environment="production"}[5m])) > 100
   ```
   - Severity: Warning
   - Duration: 10 minutes
   - Action: Investigate processing bottlenecks

---

## Dashboard Maintenance

### Regular Tasks

1. **Weekly Review**
   - Check for new metric labels
   - Update variable queries
   - Verify alert thresholds

2. **After Deployments**
   - Verify all panels load
   - Check for metric name changes
   - Test variable filters

3. **Quarterly Optimization**
   - Remove unused panels
   - Optimize query performance
   - Update documentation

### Version Control

All dashboard JSON files are version-controlled in this repository:
```
docs/
  grafana-overview-dashboard.json
  grafana-pipeline-dashboard.json
  grafana-orderbook-dashboard.json
  grafana-pool-dashboard.json
  GRAFANA_DASHBOARDS.md (this file)
```

---

## Troubleshooting

### Variables Show "$environment" Literally or "No Options Found"

This is the most common issue after importing dashboards.

**Root Cause**: Template variables need to be properly initialized with a datasource.

**Solution:**
1. Click the gear icon (⚙️) at the top right → **Dashboard Settings**
2. Go to **Variables** in the left sidebar
3. For each variable (environment, pool_name, etc.):
   - Click the variable name to edit it
   - In the "Data source" field, **explicitly select your Prometheus datasource** (don't leave it as "${datasource}")
   - Click "Update" and then "Save dashboard"
4. Alternatively, at the top of the dashboard, use the "Datasource" dropdown to select your Prometheus instance
5. Refresh the dashboard (Ctrl+R or click the refresh icon)

**Quick Fix - Edit Variables:**
```
Dashboard Settings → Variables → 
  - Click "environment" → Set datasource to your Prometheus → Update
  - Click "pool_name" → Set datasource to your Prometheus → Update
  - Click "object_type" → Set datasource to your Prometheus → Update
```

### Dashboards Show "No Data"

1. **Check Prometheus is scraping metrics**
   ```bash
   # Check targets
   curl http://prometheus:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="otel-collector")'
   ```

2. **Verify gateway is exporting metrics**
   ```bash
   # Check OpenTelemetry Collector metrics endpoint
   curl http://otel-collector:8889/metrics | grep meltica
   ```

3. **Test metric availability in Prometheus**
   ```bash
   # Query Prometheus directly for pool metrics
   curl 'http://prometheus:9090/api/v1/query?query=pool_objects_active'
   
   # Check for environment labels
   curl 'http://prometheus:9090/api/v1/label/environment/values'
   ```

4. **Check variable values**
   - At the top of the dashboard, check what's selected in each dropdown
   - Try selecting "All" for environment/pool_name/object_type
   - If dropdowns are empty, the variables aren't querying correctly (see above)

### Variables Show Wrong Values

If variables populate but with unexpected values:

1. **Check label names in your metrics**
   ```bash
   # See all labels on pool metrics
   curl 'http://prometheus:9090/api/v1/series?match[]=pool_objects_active' | jq
   ```

2. **Update variable queries if needed**
   - Go to Dashboard Settings → Variables
   - Check the "Query" field for each variable
   - Ensure it matches your actual metric and label names

### Panels Show Errors

- **"Parse error"**: Check PromQL syntax in panel query
- **"Timeout"**: Query is too expensive, reduce time range or add filters
- **"Too many samples"**: Increase Prometheus query timeout or use rate() aggregation
- **"Template variables could not be initialized"**: Datasource not selected, see "Variables Show $environment" above

### Metrics Have Different Labels

If your deployment uses different label names:
1. Export dashboard JSON
2. Find/replace label names globally
3. Re-import dashboard

Example: Replacing `environment` with `env`:
```bash
sed -i 's/environment=~/env=~/g' grafana-overview-dashboard.json
sed -i 's/"environment"/"env"/g' grafana-overview-dashboard.json
```

---

## Advanced Query Examples

### Event Success Rate
```promql
100 * (
  sum(rate(databus_events_published{environment="production"}[5m]))
  /
  sum(rate(dispatcher_events_ingested{environment="production"}[5m]))
)
```

### Pool Pressure Score
```promql
100 * (
  1 - (
    pool_available{environment="production"}
    /
    pool_capacity{environment="production"}
  )
)
```

### Orderbook Health Score
```promql
100 * (
  1 - (
    rate(orderbook_gap_detected{environment="production"}[5m])
    /
    (rate(orderbook_snapshot_applied{environment="production"}[5m]) + 0.001)
  )
)
```

### Provider Comparison
```promql
sum by (provider) (
  rate(dispatcher_events_ingested{environment="production"}[5m])
)
/
sum(
  rate(dispatcher_events_ingested{environment="production"}[5m])
)
```

---

## Next Steps

1. **Import Dashboards**: Start with System Overview
2. **Configure Alerts**: Set up critical alerts in Grafana or Alertmanager
3. **Create Runbooks**: Document response procedures for each alert
4. **Train Team**: Ensure operations team understands each dashboard
5. **Customize**: Adapt panels and thresholds to your specific needs

For questions or improvements, see the main [README.md](../README.md).
