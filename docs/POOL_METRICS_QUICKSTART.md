# Pool Metrics Quick Start Guide

Get your pool metrics dashboard up and running in Grafana in 5 minutes.

## Step 1: Verify Metrics are Being Exported

Check that the gateway is exporting metrics:

```bash
# If using OTLP exporter, check Prometheus scrape target
curl http://localhost:9090/api/v1/targets | jq

# Or check your OpenTelemetry Collector
curl http://localhost:8889/metrics | grep pool
```

**Expected metrics:**
```
pool_objects_borrowed{pool_name="CanonicalEvent",object_type="*schema.Event"} 1234
pool_objects_returned{pool_name="CanonicalEvent",object_type="*schema.Event"} 1230
pool_objects_active{pool_name="CanonicalEvent",object_type="*schema.Event"} 4
pool_capacity{pool_name="CanonicalEvent",object_type="*schema.Event"} 1000
pool_available{pool_name="CanonicalEvent",object_type="*schema.Event"} 996
pool_utilization{pool_name="CanonicalEvent",object_type="*schema.Event"} 0.004
```

## Step 2: Import Dashboard into Grafana

### Option A: Import JSON (Recommended)

1. Open Grafana → Dashboards → Import
2. Upload `docs/grafana-pool-dashboard.json`
3. Select your Prometheus datasource
4. Click "Import"

### Option B: Create Manually

Follow the detailed instructions in `docs/grafana-pool-dashboard.md`

## Step 3: Configure Prometheus Data Source

If not already configured:

1. Go to Configuration → Data Sources → Add data source
2. Select "Prometheus"
3. Set URL: `http://localhost:9090` (or your Prometheus endpoint)
4. Click "Save & Test"

## Step 4: Test Your Dashboard

Once imported, you should see:

✅ **Total Active Objects** - Should show current count  
✅ **Pool Utilization** - Bar gauge for each pool (0-100%)  
✅ **Active Objects Over Time** - Graph showing usage trends  
✅ **Pool Details Table** - Complete overview of all pools  

## Step 5: Set Up Alerts (Optional)

### Quick Alert: Pool Exhaustion

1. Open "Available Objects" panel
2. Click "Alert" tab
3. Configure:
   - **Condition**: WHEN `avg()` OF `query(A, 5m, now)` IS BELOW `1`
   - **Frequency**: Evaluate every `1m` for `5m`
   - **Notification**: Select your alert channel

Or use the pre-configured alert in the imported dashboard.

## Common Queries to Test

### 1. See all pools
```promql
pool_utilization
```

### 2. Check CanonicalEvent pool
```promql
pool_objects_active{pool_name="CanonicalEvent"}
```

### 3. Find pools running hot
```promql
pool_utilization > 0.8
```

### 4. Borrow latency p99
```promql
histogram_quantile(0.99, rate(pool_borrow_duration_bucket[5m]))
```

## Troubleshooting

### "No data" in panels?

**Check 1: Time range**
- Set to "Last 1 hour" or "Last 5 minutes"

**Check 2: Metric names**
- OpenTelemetry converts dots to underscores
- Use `pool_utilization` not `pool.utilization`

**Check 3: Label format**
```promql
# Correct ✅
pool_utilization{pool_name="CanonicalEvent"}

# Wrong ❌
pool_utilization{pool.name="CanonicalEvent"}
```

**Check 4: Gateway running**
```bash
# Check if gateway is running
ps aux | grep gateway

# Check logs for telemetry initialization
tail -f gateway.log | grep telemetry
```

**Check 5: Prometheus scraping**
```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="meltica")'
```

### Metrics exist but panels are empty?

**Check datasource variable:**
- Dashboard settings → Variables → datasource
- Make sure it's pointing to your Prometheus instance

**Check label matchers:**
- Remove filters if using variables: Set `$pool_name` to "All"

## Next Steps

### 1. Customize for Your Needs

Edit queries to focus on critical pools:
```promql
# Focus on your most important pool
pool_utilization{pool_name="CanonicalEvent"}
```

### 2. Add More Alerts

- High utilization (> 90%)
- Low available objects (< 10)
- Borrow latency spike
- Memory leak detection (active objects never decreasing)

See `docs/grafana-pool-dashboard.md` for alert examples.

### 3. Create Team Dashboards

- **Dev Dashboard**: Focus on CanonicalEvent and WsFrame
- **Ops Dashboard**: All pools with capacity planning
- **Alerts Dashboard**: Only high-priority metrics

### 4. Export and Version Control

```bash
# Export your customized dashboard
# Grafana → Dashboard → Settings → JSON Model → Copy to clipboard

# Save to repo
vim docs/grafana-pool-dashboard-custom.json
```

## Key Metrics to Watch

| Metric | What to Monitor | Alert When |
|--------|----------------|------------|
| `pool_utilization` | Resource usage | > 90% for 5 min |
| `pool_available` | Capacity left | < 10 objects |
| `pool_borrow_duration` | Performance | p99 > 100ms |
| `pool_objects_active` | Memory leaks | Continuously rising |

## Example Alert Rules (Prometheus)

Add to your `prometheus.yml` alerts:

```yaml
groups:
  - name: pool_alerts
    interval: 30s
    rules:
      - alert: PoolHighUtilization
        expr: pool_utilization > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Pool {{ $labels.pool_name }} high utilization"
          description: "Pool is at {{ $value | humanizePercentage }} capacity"

      - alert: PoolExhausted
        expr: pool_available == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Pool {{ $labels.pool_name }} EXHAUSTED"
          description: "No objects available in pool!"

      - alert: HighBorrowLatency
        expr: |
          histogram_quantile(0.99, 
            rate(pool_borrow_duration_bucket[5m])
          ) > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High borrow latency for {{ $labels.pool_name }}"
          description: "p99 latency is {{ $value }}ms"
```

## Resources

- **Full Guide**: `docs/grafana-pool-dashboard.md`
- **Dashboard JSON**: `docs/grafana-pool-dashboard.json`
- **Metrics Examples**: `docs/pool-metrics-example.md`

## Support

If metrics aren't showing:

1. Check gateway logs: `tail -f gateway.log`
2. Verify telemetry config in code
3. Test Prometheus scrape: `curl http://localhost:9090/api/v1/query?query=pool_utilization`
4. Check OpenTelemetry Collector logs

Happy monitoring! 🚀
