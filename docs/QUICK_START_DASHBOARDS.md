# Quick Start: Get Grafana Dashboards Working in 5 Minutes

## Prerequisites

You need:
- ✅ Gateway running
- ✅ OpenTelemetry Collector running
- ✅ Prometheus running
- ✅ Grafana running

---

## Step 1: Run the Diagnostic Script (2 minutes)

This will tell you exactly what's wrong:

```bash
cd /home/qing/work/meltica
./scripts/check-metrics.sh
```

**Read the output carefully!** It will show you:
- ✓ What's working
- ✗ What's broken
- → What to do next

---

## Step 2: Fix Common Issues

### Issue A: Gateway not running or not exporting metrics

**Symptoms from diagnostic:**
```
✗ Gateway process NOT found
✗ No 'environment' label found
```

**Fix:**
```bash
# Make sure OTEL endpoint is set
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_RESOURCE_ATTRIBUTES=environment=development,service.name=meltica

# Run the gateway
./bin/gateway
```

### Issue B: Metrics not in Prometheus

**Symptoms from diagnostic:**
```
✓ OpenTelemetry Collector is reachable
✗ pool_objects_active has NO data or doesn't exist
```

**Fix:**

Check Prometheus is configured to scrape OTel Collector:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['localhost:8889']
```

Restart Prometheus after config change.

### Issue C: Metric names are different

**Symptoms from diagnostic:**
```
Found: meltica_pool_objects_active
Found: meltica_dispatcher_events_ingested
```

Your metrics have a `meltica_` prefix!

**Fix:**

Update dashboard JSON files before importing:

```bash
cd /home/qing/work/meltica/docs

# Add meltica_ prefix to all metrics
sed -i 's/pool_objects_active/meltica_pool_objects_active/g' grafana-pool-dashboard.json
sed -i 's/pool_capacity/meltica_pool_capacity/g' grafana-pool-dashboard.json
sed -i 's/pool_utilization/meltica_pool_utilization/g' grafana-pool-dashboard.json
sed -i 's/pool_available/meltica_pool_available/g' grafana-pool-dashboard.json
sed -i 's/pool_objects_borrowed/meltica_pool_objects_borrowed/g' grafana-pool-dashboard.json
sed -i 's/pool_objects_returned/meltica_pool_objects_returned/g' grafana-pool-dashboard.json
sed -i 's/pool_borrow_duration/meltica_pool_borrow_duration/g' grafana-pool-dashboard.json

# Repeat for other dashboards if needed
```

---

## Step 3: Verify Metrics in Prometheus UI (1 minute)

**Before configuring Grafana, make sure queries work in Prometheus!**

1. Open Prometheus: http://localhost:9090
2. Click **"Graph"** tab
3. Type this query:
   ```promql
   pool_objects_active
   ```
   (or `meltica_pool_objects_active` if you have the prefix)
4. Click **"Execute"**

**Expected result:** You should see a table with values like:
```
pool_objects_active{environment="development", pool_name="Event", ...} 10
pool_objects_active{environment="development", pool_name="OrderRequest", ...} 5
```

**If you see "No data"**: Go back to Step 2, metrics aren't in Prometheus yet.

**If you see data**: Great! Move to Step 4.

---

## Step 4: Import Dashboard to Grafana (1 minute)

1. Open Grafana: http://localhost:3000
2. Go to **Dashboards → Import**
3. Click **"Upload JSON file"**
4. Select: `/home/qing/work/meltica/docs/grafana-pool-dashboard.json`
5. In the import dialog:
   - **Datasource dropdown**: Select your Prometheus instance
6. Click **"Import"**

---

## Step 5: Configure Dashboard Variables (1 minute)

After import, the dashboard opens but might show "No data".

### Quick Fix:

1. At the top of the dashboard, find the **"Datasource"** dropdown
2. Select your Prometheus instance from the dropdown
3. The other dropdowns (Environment, Pool Name, Object Type) should now populate
4. Select **"All"** for Environment, Pool Name, and Object Type
5. Hit **Refresh** (circular arrow icon top right)

### If dropdowns still show "No options found":

**Manual Variable Configuration (do this once per dashboard):**

1. Click **gear icon (⚙️)** top right → **Dashboard settings**
2. Click **"Variables"** in left sidebar
3. Click **"environment"** variable
4. In **"Data source"** field, change from `${datasource}` to your actual Prometheus instance name
5. Click **"Update"** (bottom of page)
6. Repeat steps 3-5 for **"pool_name"** and **"object_type"** variables
7. Click **"Save dashboard"** (floppy disk icon)
8. Click **"Back to dashboard"**
9. Now the dropdowns should populate with values
10. Select **"All"** for each dropdown

---

## Step 6: Verify Dashboard Shows Data

You should now see:
- **Top row stats**: Numbers with colored backgrounds
- **Charts**: Lines showing metrics over time
- **Bottom table**: Rows with pool details

**If you still see "No data":**

1. Check the time range (top right) - try "Last 15 minutes"
2. Check variable selections - make sure "All" is selected
3. Open Prometheus UI and verify this query works:
   ```promql
   sum(pool_objects_active{environment=~".*"})
   ```
   If this returns 0 or nothing, your gateway hasn't created any pool objects yet.

---

## Step 7: Import Other Dashboards (optional)

Repeat Steps 4-5 for:
- `grafana-overview-dashboard.json` - System overview
- `grafana-pipeline-dashboard.json` - Event pipeline
- `grafana-orderbook-dashboard.json` - Orderbook metrics

---

## Troubleshooting Decision Tree

```
Is gateway running?
  NO → Start gateway with OTEL env vars
  YES ↓

Are metrics at http://localhost:8889/metrics?
  NO → Check OTEL_EXPORTER_OTLP_ENDPOINT
  YES ↓

Are metrics in Prometheus UI (http://localhost:9090)?
  NO → Check Prometheus scrape config
  YES ↓

Do queries work in Prometheus UI?
  NO → Fix metric names (Step 2, Issue C)
  YES ↓

Do dashboard variables populate?
  NO → Configure variables manually (Step 5)
  YES ↓

Do panels show data?
  NO → Check time range & variable selections
  YES → Success! 🎉
```

---

## Still Stuck?

Run this and share the output:

```bash
# Full diagnostic report
./scripts/check-metrics.sh > /tmp/metrics-report.txt 2>&1

# Test a simple query
curl 'http://localhost:9090/api/v1/query?query=pool_objects_active' | jq

# Check Grafana variable query
curl 'http://localhost:9090/api/v1/label/environment/values' | jq
```

Then share:
1. `/tmp/metrics-report.txt`
2. Screenshot of Grafana dashboard showing the issue
3. Screenshot of Grafana Dashboard Settings → Variables

---

## Success Checklist

- [ ] Diagnostic script shows ✓ for all checks
- [ ] Prometheus UI shows data for `pool_objects_active`
- [ ] Dashboard variables have options (not "No options found")
- [ ] At least one panel shows a chart with data
- [ ] Changing time range updates the charts
- [ ] Changing variable selections updates the panels

**Once all checked: You're done!** 🎉
