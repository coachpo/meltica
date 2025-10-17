# Pool Metrics Documentation

Complete guide for monitoring Meltica pool metrics in Grafana.

## 📚 Documentation Index

| Document | Purpose | Audience |
|----------|---------|----------|
| **[POOL_METRICS_QUICKSTART.md](./POOL_METRICS_QUICKSTART.md)** | Get started in 5 minutes | Everyone |
| **[grafana-pool-dashboard.md](./grafana-pool-dashboard.md)** | Detailed panel configurations | Dashboard builders |
| **[grafana-pool-dashboard.json](./grafana-pool-dashboard.json)** | Ready-to-import dashboard | Quick setup |
| **[pool-metrics-example.md](./pool-metrics-example.md)** | Metric reference & examples | Developers |
| **[pool-metrics-architecture.md](./pool-metrics-architecture.md)** | Data flow & architecture | DevOps/SRE |

## 🚀 Quick Start

1. **Verify metrics are being exported**
   ```bash
   curl http://localhost:8889/metrics | grep pool
   ```

2. **Import dashboard into Grafana**
   - Open Grafana → Dashboards → Import
   - Upload `grafana-pool-dashboard.json`
   - Select Prometheus datasource

3. **View your metrics!**
   - Open "Meltica Pool Metrics" dashboard
   - Set time range to "Last 1 hour"
   - Filter by pool name or object type

📖 Full instructions: [POOL_METRICS_QUICKSTART.md](./POOL_METRICS_QUICKSTART.md)

## 📊 Available Metrics

All metrics include `pool.name` and `object.type` labels:

### Counters
- `pool.objects.borrowed` - Total objects borrowed
- `pool.objects.returned` - Total objects returned

### Gauges  
- `pool.objects.active` - Currently borrowed objects
- `pool.capacity` - Pool capacity (static)
- `pool.available` - Available objects now
- `pool.utilization` - Usage ratio (0.0-1.0)

### Histograms
- `pool.borrow.duration` - Time to acquire objects (ms)

## 🎯 Common Use Cases

### Monitor pool health
```promql
pool_utilization{pool_name="CanonicalEvent"}
```

### Find pools running out
```promql
pool_available < 10
```

### Check borrow performance
```promql
histogram_quantile(0.99, rate(pool_borrow_duration_bucket[5m]))
```

### Detect memory leaks
```promql
delta(pool_objects_active[10m]) > 0
```

📖 More examples: [pool-metrics-example.md](./pool-metrics-example.md)

## 🎨 Dashboard Preview

The included dashboard provides:

**Top Row** - Key metrics (4 stat panels)
- Total Active Objects
- Total Capacity
- Average Utilization
- Borrow Rate

**Bar Gauge** - Pool utilization by type with color coding

**Time Series** - 4 graphs showing:
- Active objects over time (vs capacity)
- Borrow/return rates
- Latency percentiles (p50, p95, p99)
- Available objects

**Table** - Complete pool details with colored cells

📖 Detailed configuration: [grafana-pool-dashboard.md](./grafana-pool-dashboard.md)

## ⚠️ Recommended Alerts

### Critical
- **Pool Exhaustion**: `pool_available == 0` for 1 minute
- **High Utilization**: `pool_utilization > 0.9` for 5 minutes

### Warning
- **Low Availability**: `pool_available < 10`
- **High Latency**: `p99 > 100ms` for 5 minutes
- **Memory Leak**: Active objects continuously rising

📖 Alert configuration: [grafana-pool-dashboard.md](./grafana-pool-dashboard.md#alerting-rules)

## 🏗️ Architecture

```
Gateway → OTLP Collector → Prometheus → Grafana
         (metrics)         (storage)    (visualization)
```

📖 Detailed flow: [pool-metrics-architecture.md](./pool-metrics-architecture.md)

## 🔧 Troubleshooting

### Metrics not showing?

1. **Check gateway telemetry**
   ```bash
   # Gateway logs should show:
   # "telemetry initialized: endpoint=http://localhost:4318"
   ```

2. **Check collector receiving**
   ```bash
   curl http://localhost:8889/metrics | grep pool
   ```

3. **Check Prometheus scraping**
   ```bash
   curl http://localhost:9090/api/v1/targets | jq
   ```

4. **Check Grafana datasource**
   - Configuration → Data Sources → Prometheus
   - Click "Save & Test"

📖 Full troubleshooting: [POOL_METRICS_QUICKSTART.md](./POOL_METRICS_QUICKSTART.md#troubleshooting)

## 📝 Gateway Pools

| Pool Name | Object Type | Capacity | Purpose |
|-----------|-------------|----------|---------|
| WsFrame | *schema.WsFrame | 200 | WebSocket frames |
| ProviderRaw | *schema.ProviderRaw | 200 | Raw provider data |
| CanonicalEvent | *schema.Event | 1000 | Canonical events |
| OrderRequest | *schema.OrderRequest | 20 | Order requests |
| ExecReport | *schema.ExecReport | 20 | Execution reports |

## 💡 Tips

1. **Start with filters**: Use `$pool_name` and `$object_type` variables
2. **Focus on critical pools**: CanonicalEvent (largest) and OrderRequest (trading)
3. **Set up alerts early**: Don't wait for production issues
4. **Use annotations**: Mark deployments on graphs
5. **Export dashboards**: Version control your customizations

## 🤝 Contributing

Found a useful query or panel? Add it to:
- `grafana-pool-dashboard.md` for documentation
- `grafana-pool-dashboard.json` for the dashboard itself

## 📞 Support

Issues with metrics?

1. Check gateway logs: `tail -f gateway.log`
2. Test metrics endpoint: `curl http://localhost:8889/metrics`
3. Verify Prometheus query: `curl 'http://localhost:9090/api/v1/query?query=pool_utilization'`
4. Check Grafana query inspector: Panel → Inspect → Query

---

**Quick Links:**
- 🚀 [Quick Start](./POOL_METRICS_QUICKSTART.md)
- 📊 [Dashboard Guide](./grafana-pool-dashboard.md)
- 📦 [Import Dashboard](./grafana-pool-dashboard.json)
- 📖 [Metric Reference](./pool-metrics-example.md)
- 🏗️ [Architecture](./pool-metrics-architecture.md)
