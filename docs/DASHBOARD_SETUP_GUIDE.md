# Grafana Dashboard Setup - Step-by-Step Troubleshooting Guide

## Step 1: Verify Metrics Are Being Collected

First, let's check if metrics are actually reaching Prometheus.

### 1.1 Check if the Gateway is Running and Exporting Metrics

```bash
# Check if the gateway process is running
ps aux | grep gateway

# Check environment variable for OTEL endpoint
echo $OTEL_EXPORTER_OTLP_ENDPOINT

# If running in Docker, check container logs
docker logs <gateway-container-name> | grep -i metric
```

### 1.2 Check OpenTelemetry Collector Metrics Endpoint

```bash
# Check if OTel Collector is exposing metrics
curl http://localhost:8889/metrics | head -20

# Look for meltica-specific metrics
curl http://localhost:8889/metrics | grep -E "pool|dispatcher|orderbook|databus"

# Count how many meltica metrics exist
curl -s http://localhost:8889/metrics | grep -E "pool|dispatcher|orderbook|databus" | wc -l
```

**Expected output**: You should see metrics like:
```
pool_objects_active{...} 10
pool_capacity{...} 100
dispatcher_events_ingested{...} 1234
```

### 1.3 Check Prometheus is Scraping

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job | contains("otel"))'

# Check if Prometheus has any pool metrics
curl 'http://localhost:9090/api/v1/query?query=pool_objects_active' | jq

# List all metric names in Prometheus
curl http://localhost:9090/api/v1/label/__name__/values | jq '.data[] | select(. | contains("pool") or contains("dispatcher"))'
```

**Expected output**: Should show active targets and metric results.

---

## Step 2: Identify Your Actual Metric Names

The metric names in your system might be slightly different. Let's find out:

```bash
# Get ALL metric names (pipe to file if too many)
curl -s http://localhost:9090/api/v1/label/__name__/values | jq -r '.data[]' | sort > /tmp/all_metrics.txt

# Search for relevant metrics
grep -E "pool|dispatcher|orderbook|databus" /tmp/all_metrics.txt
```

### Common Metric Name Variations:

| Expected Name | Possible Alternatives |
|---------------|----------------------|
| `pool_objects_active` | `pool.objects.active`, `meltica_pool_objects_active` |
| `dispatcher_events_ingested` | `dispatcher.events.ingested`, `meltica_dispatcher_events_ingested` |
| `orderbook_gap_detected` | `orderbook.gap.detected`, `meltica_orderbook_gap_detected` |
| `databus_events_published` | `databus.events.published`, `meltica_databus_events_published` |

---

## Step 3: Check Metric Labels

Once you find a metric, check what labels it has:

```bash
# Check labels on pool metrics
curl 'http://localhost:9090/api/v1/series?match[]=pool_objects_active' | jq

# Check if 'environment' label exists
curl 'http://localhost:9090/api/v1/label/environment/values' | jq

# See all labels on a specific metric
curl 'http://localhost:9090/api/v1/series?match[]=pool_objects_active' | jq '.data[0]'
```

**Expected output**:
```json
{
  "__name__": "pool_objects_active",
  "environment": "development",
  "pool_name": "Event",
  "object_type": "*schema.Event",
  "service": "meltica",
  "instance": "otel-collector:8889",
  "job": "otel-collector"
}
```

### If 'environment' label is missing:

Your metrics might not have the `environment` label. Check what labels you DO have:

```bash
# List all label names
curl http://localhost:9090/api/v1/labels | jq
```

Common alternatives: `env`, `deployment`, `stage`, `namespace`

---

## Step 4: Test PromQL Queries Manually

Before using dashboards, test queries directly in Prometheus:

1. **Open Prometheus UI**: http://localhost:9090
2. **Go to Graph tab**
3. **Try these queries one by one**:

### Test Query 1: Basic Metric Existence
```promql
pool_objects_active
```
**Expected**: Should return values. If not, metric doesn't exist or has different name.

### Test Query 2: With Label Filter (if environment label exists)
```promql
pool_objects_active{environment="development"}
```
**Expected**: Should return values for that environment.

### Test Query 3: With Regex Filter (what dashboard uses)
```promql
pool_objects_active{environment=~".*"}
```
**Expected**: Should return all environments (equivalent to "All").

### Test Query 4: Aggregation
```promql
sum(pool_objects_active)
```
**Expected**: Should return total count across all pools.

### Test Query 5: Rate Calculation
```promql
rate(pool_objects_borrowed[1m])
```
**Expected**: Should show borrow rate per second.

---

## Step 5: Configure Grafana Dashboard Variables

Once you know your metrics work, configure the dashboard:

### 5.1 Import Dashboard
1. Grafana → Dashboards → Import
2. Upload `grafana-pool-dashboard.json`
3. **Select your Prometheus datasource** in the import dialog
4. Click Import

### 5.2 Check Variable Configuration

After import, you should see dropdowns at the top. If they're empty:

1. Click **gear icon (⚙️)** at top right → **Dashboard settings**
2. Click **Variables** in left sidebar
3. **For EACH variable**, click it and verify:

#### Datasource Variable:
```
Type: Datasource
Query: prometheus
Current: [Your Prometheus instance]
```

#### Environment Variable:
```
Type: Query
Query: label_values(pool_objects_active, environment)
Data source: [Select your Prometheus - NOT ${datasource}]
Multi-value: checked
Include All option: checked
Refresh: On Dashboard Load
```

**CRITICAL**: The "Data source" field must have your actual Prometheus selected, not "${datasource}"

#### Pool Name Variable:
```
Type: Query
Query: label_values(pool_objects_active{environment=~"$environment"}, pool_name)
Data source: [Your Prometheus]
Multi-value: checked
Include All option: checked
```

#### Object Type Variable:
```
Type: Query
Query: label_values(pool_objects_active{environment=~"$environment"}, object_type)
Data source: [Your Prometheus]
Multi-value: checked
Include All option: checked
```

4. Click **Update** for each variable
5. Click **Save dashboard** (floppy disk icon top right)
6. Click **Back to dashboard**

### 5.3 Select Variable Values

At the top of the dashboard:
1. **Datasource dropdown**: Select your Prometheus
2. **Environment dropdown**: Should now show values (development, production, etc.) - Select "All" or specific one
3. **Pool Name dropdown**: Should show "Event", "OrderRequest", etc. - Select "All"
4. **Object Type dropdown**: Should show "*schema.Event", etc. - Select "All"

---

## Step 6: If Metrics Use Different Names

If your metrics have different names (e.g., `meltica_pool_objects_active` instead of `pool_objects_active`), you need to update the dashboard:

### Option A: Edit JSON Before Import

1. **Before importing**, open the JSON file
2. **Find and replace** metric names:
   ```bash
   # Example: Add 'meltica_' prefix to all metrics
   sed -i 's/pool_objects_active/meltica_pool_objects_active/g' grafana-pool-dashboard.json
   sed -i 's/pool_capacity/meltica_pool_capacity/g' grafana-pool-dashboard.json
   sed -i 's/pool_utilization/meltica_pool_utilization/g' grafana-pool-dashboard.json
   # ... repeat for all metrics
   ```
3. **Then import** the modified JSON

### Option B: Edit After Import (Manual)

1. Go to dashboard
2. Click on a panel title → **Edit**
3. Update the query with the correct metric name
4. Repeat for all panels (tedious but works)

---

## Step 7: Common Issues and Fixes

### Issue: Variables show "No options found"

**Cause**: Variable queries can't reach Prometheus or return no data.

**Fix**:
```bash
# Test the variable query directly in Prometheus UI
label_values(pool_objects_active, environment)

# If this returns nothing, either:
# 1. Metric doesn't exist (check Step 2)
# 2. Metric has no data (check if gateway is running)
# 3. Label name is different (check Step 3)
```

### Issue: Panels show "No data"

**Cause**: Query is valid but returns no results.

**Fix**:
1. Click panel title → **Edit**
2. Look at the query
3. Copy it and test in Prometheus UI (http://localhost:9090)
4. Simplify query to debug:
   ```promql
   # Start simple
   pool_objects_active
   
   # Add filters one by one
   pool_objects_active{environment="development"}
   pool_objects_active{environment="development", pool_name="Event"}
   ```

### Issue: Variables show "$environment" literally in panel

**Cause**: Variable isn't being substituted.

**Fix**:
- Dashboard Settings → Variables → Check variable name is exactly "environment" (case-sensitive)
- In panel query, use `$environment` not `${environment}`
- Or use `[[environment]]` syntax

### Issue: Metric names have dots instead of underscores

**Cause**: OpenTelemetry exports with dots (e.g., `pool.objects.active`) but Prometheus converts to underscores.

**Fix**: Use underscores in queries: `pool_objects_active`

### Issue: Need to change label name globally

```bash
# Example: Replace 'environment' label with 'env'
sed -i 's/environment=~/env=~/g' grafana-pool-dashboard.json
sed -i 's/"environment"/"env"/g' grafana-pool-dashboard.json
sed -i 's/\$environment/\$env/g' grafana-pool-dashboard.json

# Then re-import the modified dashboard
```

---

## Step 8: Verification Checklist

Before declaring success, verify:

- [ ] Gateway is running and exporting metrics
- [ ] OpenTelemetry Collector is exposing metrics on :8889
- [ ] Prometheus is scraping the collector
- [ ] At least one metric (e.g., `pool_objects_active`) returns data in Prometheus UI
- [ ] Dashboard variables populate with options (not "No options found")
- [ ] Selecting variable values updates panels
- [ ] At least one panel shows data (not "No data")

---

## Quick Diagnostic Script

Run this to get all the info you need:

```bash
#!/bin/bash
echo "=== Gateway Process ==="
ps aux | grep gateway | grep -v grep

echo -e "\n=== OTEL Endpoint ==="
echo $OTEL_EXPORTER_OTLP_ENDPOINT

echo -e "\n=== OTel Collector Metrics (sample) ==="
curl -s http://localhost:8889/metrics 2>/dev/null | grep -E "pool|dispatcher" | head -5

echo -e "\n=== Prometheus Targets (OTel) ==="
curl -s http://localhost:9090/api/v1/targets 2>/dev/null | jq '.data.activeTargets[] | select(.labels.job | contains("otel")) | {job: .labels.job, health: .health, lastScrape: .lastScrape}'

echo -e "\n=== Pool Metrics in Prometheus ==="
curl -s 'http://localhost:9090/api/v1/query?query=pool_objects_active' 2>/dev/null | jq '.data.result[0]'

echo -e "\n=== Available Metric Names ==="
curl -s http://localhost:9090/api/v1/label/__name__/values 2>/dev/null | jq -r '.data[]' | grep -E "pool|dispatcher|orderbook|databus" | head -10

echo -e "\n=== Environment Label Values ==="
curl -s 'http://localhost:9090/api/v1/label/environment/values' 2>/dev/null | jq

echo -e "\n=== Sample Metric Labels ==="
curl -s 'http://localhost:9090/api/v1/series?match[]=pool_objects_active' 2>/dev/null | jq '.data[0]'
```

Save as `debug-metrics.sh`, run with `bash debug-metrics.sh` and share the output for further help.

---

## Need More Help?

If you're still stuck, provide:

1. Output of the diagnostic script above
2. Screenshot of Grafana showing the empty dashboard
3. Screenshot of Dashboard Settings → Variables showing your configuration
4. One working PromQL query that returns data in Prometheus UI

This will help identify the exact issue!
