# Grafana Pool Metrics Dashboard Guide

This guide shows how to visualize the pool metrics in Grafana with example queries and panel configurations.

## Prerequisites

- Grafana with Prometheus data source configured
- OpenTelemetry Collector exporting metrics to Prometheus
- Gateway running with telemetry enabled

## Dashboard Layout

Recommended dashboard structure:

```
┌─────────────────────────────────────────────────────────┐
│  Pool Overview                                          │
│  [Total Active Objects] [Total Capacity] [Avg Util]    │
├─────────────────────────────────────────────────────────┤
│  Pool Utilization by Type                               │
│  [Gauge Panel - All Pools]                              │
├─────────────────────────────────────────────────────────┤
│  Active Objects Over Time    │  Borrow Rate             │
│  [Time Series]                │  [Time Series]           │
├─────────────────────────────────────────────────────────┤
│  Borrow Latency (p99)         │  Available Objects      │
│  [Time Series]                │  [Time Series]           │
├─────────────────────────────────────────────────────────┤
│  Pool Details Table                                     │
│  [Table Panel]                                          │
└─────────────────────────────────────────────────────────┘
```

## Panel Configurations

### 1. Total Active Objects (Stat Panel)

**Query:**
```promql
sum(pool_objects_active)
```

**Configuration:**
- Visualization: Stat
- Value: Last (not null)
- Unit: short
- Color: Based on thresholds
  - Green: 0-500
  - Yellow: 500-1000
  - Red: >1000

---

### 2. Pool Utilization by Type (Gauge Panel)

**Query:**
```promql
pool_utilization
```

**Configuration:**
- Visualization: Gauge
- Legend: `{{pool_name}} ({{object_type}})`
- Unit: percentunit (0.0-1.0)
- Thresholds:
  - Green: 0-0.7
  - Yellow: 0.7-0.9
  - Red: 0.9-1.0
- Min: 0
- Max: 1

**Alternative - Bar Gauge (Better for Multiple Pools):**
```promql
pool_utilization
```
- Visualization: Bar gauge
- Display mode: LCD
- Orientation: Horizontal

---

### 3. Active Objects Over Time (Time Series)

**Query:**
```promql
pool_objects_active
```

**Configuration:**
- Visualization: Time series
- Legend: `{{pool_name}} ({{object_type}})`
- Draw style: Line
- Line width: 2
- Fill opacity: 10
- Unit: short
- Show legend: true

**Add reference line for capacity:**
```promql
pool_capacity
```
- Display: Points
- Line style: Dashed

---

### 4. Borrow Rate (Time Series)

**Query (rate per second):**
```promql
rate(pool_objects_borrowed[1m])
```

**Configuration:**
- Visualization: Time series
- Legend: `{{pool_name}}`
- Unit: ops (operations per second)
- Y-axis label: "Borrows/sec"

---

### 5. Borrow Latency p99 (Time Series)

**Query:**
```promql
histogram_quantile(0.99, 
  rate(pool_borrow_duration_bucket[5m])
)
```

**Configuration:**
- Visualization: Time series
- Legend: `{{pool_name}} - p99`
- Unit: milliseconds (ms)
- Y-axis label: "Latency (ms)"
- Connect null values: true

**Add p95 and p50 for comparison:**
```promql
# p95
histogram_quantile(0.95, rate(pool_borrow_duration_bucket[5m]))

# p50 (median)
histogram_quantile(0.50, rate(pool_borrow_duration_bucket[5m]))
```

---

### 6. Available Objects (Time Series)

**Query:**
```promql
pool_available
```

**Configuration:**
- Visualization: Time series
- Legend: `{{pool_name}}`
- Unit: short
- Fill opacity: 20
- Alert when value drops to 0

---

### 7. Pool Details Table (Table Panel)

**Queries (combine multiple):**

A. Current state:
```promql
pool_objects_active
```

B. Capacity:
```promql
pool_capacity
```

C. Available:
```promql
pool_available
```

D. Utilization:
```promql
pool_utilization
```

E. Borrow rate:
```promql
rate(pool_objects_borrowed[5m])
```

**Configuration:**
- Visualization: Table
- Transform: Join by labels (pool_name, object_type)
- Column settings:
  - Pool Name: `pool_name`
  - Type: `object_type`
  - Active: Query A (Unit: short)
  - Capacity: Query B (Unit: short)
  - Available: Query C (Unit: short)
  - Utilization: Query D (Unit: percentunit)
  - Borrow Rate: Query E (Unit: ops)
- Sort by: Utilization (descending)
- Color cells by value

---

## Advanced Queries

### Pool Exhaustion Detection
```promql
# Pools with no available objects
pool_available == 0
```

### Type-Level Aggregation
```promql
# Total active objects by type across all pools
sum by (object_type) (pool_objects_active)
```

### Return Rate vs Borrow Rate
```promql
# Borrow rate
rate(pool_objects_borrowed[1m])

# Return rate
rate(pool_objects_returned[1m])
```

### Pool Health Score
```promql
# Healthy if utilization < 0.8 AND available > 10
(pool_utilization < 0.8) * (pool_available > 10)
```

### Borrow Wait Time Estimation
```promql
# Average borrow duration in last 5 minutes
rate(pool_borrow_duration_sum[5m]) / 
rate(pool_borrow_duration_count[5m])
```

---

## Alerting Rules

### 1. High Pool Utilization

**Query:**
```promql
pool_utilization > 0.9
```

**Alert Condition:**
- Threshold: > 0.9
- Duration: 5 minutes
- Severity: Warning

**Message:**
```
Pool {{pool_name}} ({{object_type}}) is at {{value}}% utilization
Available: {{pool_available}}
```

---

### 2. Pool Exhaustion

**Query:**
```promql
pool_available == 0
```

**Alert Condition:**
- Threshold: == 0
- Duration: 1 minute
- Severity: Critical

**Message:**
```
Pool {{pool_name}} is EXHAUSTED - no objects available!
Active: {{pool_objects_active}}
Capacity: {{pool_capacity}}
```

---

### 3. High Borrow Latency

**Query:**
```promql
histogram_quantile(0.99, 
  rate(pool_borrow_duration_bucket[5m])
) > 100
```

**Alert Condition:**
- Threshold: > 100ms
- Duration: 5 minutes
- Severity: Warning

**Message:**
```
Pool {{pool_name}} borrow latency p99: {{value}}ms
This may indicate pool contention or capacity issues.
```

---

### 4. Memory Leak Detection

**Query:**
```promql
# Active objects not decreasing for 10 minutes
delta(pool_objects_active[10m]) > 0
```

**Alert Condition:**
- Threshold: > 0 (continuously increasing)
- Duration: 10 minutes
- Severity: Warning

**Message:**
```
Pool {{pool_name}} active objects continuously increasing.
Possible memory leak - objects not being returned.
Current active: {{pool_objects_active}}
```

---

## Variables for Dynamic Filtering

Add these variables to your dashboard for interactive filtering:

### Pool Name Variable
```
Name: pool_name
Type: Query
Query: label_values(pool_utilization, pool_name)
Multi-value: true
Include All option: true
```

### Object Type Variable
```
Name: object_type
Type: Query
Query: label_values(pool_utilization, object_type)
Multi-value: true
Include All option: true
```

**Use in queries:**
```promql
pool_utilization{pool_name=~"$pool_name", object_type=~"$object_type"}
```

---

## Example Dashboard Row Panels

### Row 1: Key Metrics
```promql
# Stat panels side by side
1. sum(pool_objects_active)           # Total Active
2. sum(pool_capacity)                  # Total Capacity
3. avg(pool_utilization)               # Avg Utilization
4. sum(rate(pool_objects_borrowed[1m])) # Borrow Rate
```

### Row 2: CanonicalEvent Specific (Most Critical)
```promql
# Focus on the largest pool
pool_objects_active{pool_name="CanonicalEvent"}
pool_available{pool_name="CanonicalEvent"}
pool_utilization{pool_name="CanonicalEvent"}
rate(pool_objects_borrowed{pool_name="CanonicalEvent"}[1m])
```

---

## Quick Start: Import Ready Query

Copy this query into a new panel to get started:

```promql
# All pools utilization with labels
pool_utilization
```

Then configure:
1. Set visualization to "Time series" or "Gauge"
2. Add legend: `{{pool_name}}`
3. Set unit to "percentunit"
4. Add thresholds (0.7 yellow, 0.9 red)

---

## Pro Tips

1. **Use Variables**: Make dashboard reusable with `$pool_name` and `$object_type` variables
2. **Group Panels**: Use rows to organize by concern (Performance, Capacity, Errors)
3. **Link Panels**: Click on a utilization gauge → jump to detailed time series
4. **Time Range**: Default to "Last 1 hour" for real-time monitoring
5. **Auto Refresh**: Set to 30s or 1m for live dashboards
6. **Annotations**: Mark deployment times to correlate with metric changes

---

## Troubleshooting

### Metrics Not Showing?

Check:
1. **Gateway telemetry enabled**: `OTEL_EXPORTER_OTLP_ENDPOINT` set?
2. **Prometheus scraping**: Check `/metrics` endpoint
3. **Metric names**: OpenTelemetry might convert to `pool_objects_active` (underscores)
4. **Label matchers**: Use `{pool_name="CanonicalEvent"}` not `{pool.name="..."}`

### Empty Graphs?

- Check time range (last 1 hour)
- Verify metric exists: `{__name__=~"pool.*"}`
- Check if gateway is running and processing events
- Verify Prometheus is scraping metrics

---

## Next Steps

1. Import or create dashboard with these panels
2. Set up alerts for critical conditions
3. Add annotations for deployments
4. Create separate dashboards for production vs staging
5. Share dashboard JSON with team
