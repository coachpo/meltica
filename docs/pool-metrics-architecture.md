# Pool Metrics Architecture

This document explains how pool metrics flow from the gateway to your monitoring system.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Gateway Application                       │
│                                                                   │
│  ┌──────────────┐         ┌──────────────┐                      │
│  │ Pool Manager │────────▶│ Object Pools │                      │
│  │              │         │              │                      │
│  │ • Register   │         │ • WsFrame    │                      │
│  │ • Get/Put    │         │ • ProviderRaw│                      │
│  │ • Telemetry  │         │ • Canonical  │                      │
│  └──────┬───────┘         │ • OrderReq   │                      │
│         │                 │ • ExecReport │                      │
│         │                 └──────────────┘                      │
│         │                                                        │
│         │ Record metrics                                         │
│         ▼                                                        │
│  ┌────────────────────────────────────┐                        │
│  │   OpenTelemetry Metrics SDK        │                        │
│  │                                     │                        │
│  │  • pool.objects.borrowed (counter) │                        │
│  │  • pool.objects.returned (counter) │                        │
│  │  • pool.objects.active (gauge)     │                        │
│  │  • pool.capacity (observable)      │                        │
│  │  • pool.available (observable)     │                        │
│  │  • pool.utilization (observable)   │                        │
│  │  • pool.borrow.duration (histogram)│                        │
│  └─────────────────┬──────────────────┘                        │
│                    │                                             │
└────────────────────┼─────────────────────────────────────────────┘
                     │
                     │ OTLP/gRPC or OTLP/HTTP
                     ▼
      ┌──────────────────────────────┐
      │ OpenTelemetry Collector      │
      │                              │
      │ Receivers:                   │
      │  • OTLP (gRPC :4317)        │
      │  • OTLP (HTTP :4318)        │
      │                              │
      │ Processors:                  │
      │  • Batch                     │
      │  • Memory Limiter            │
      │  • Attributes                │
      │                              │
      │ Exporters:                   │
      │  • Prometheus (Remote Write) │
      │  • Prometheus Exporter       │
      └──────────────┬───────────────┘
                     │
                     │ Prometheus Remote Write
                     │ or scrape :8889/metrics
                     ▼
         ┌────────────────────────┐
         │   Prometheus Server    │
         │                        │
         │  Storage & Querying    │
         │                        │
         │  • TSDB                │
         │  • PromQL Engine       │
         │  • Alert Manager       │
         └────────────┬───────────┘
                      │
                      │ Prometheus API
                      ▼
          ┌─────────────────────────┐
          │   Grafana Dashboard     │
          │                         │
          │  • Visualizations       │
          │  • Alerts               │
          │  • Variables            │
          │  • Annotations          │
          └─────────────────────────┘
```

## Metric Attributes Flow

Every metric includes these labels/attributes:

```
pool_objects_active{
  pool_name="CanonicalEvent",     ← Pool identifier
  object_type="*schema.Event",    ← Go type name
  instance="gateway-1",            ← Added by collector
  job="meltica"                    ← Added by Prometheus
}
```

## Data Flow Example

1. **Application Event**: Gateway borrows an Event from pool
   ```go
   evt, err := poolMgr.BorrowCanonicalEvent(ctx)
   ```

2. **Metric Recording**: PoolManager records to OpenTelemetry
   ```go
   objectsBorrowedCounter.Add(ctx, 1, 
     attribute.String("pool.name", "CanonicalEvent"),
     attribute.String("object.type", "*schema.Event"))
   ```

3. **OTLP Export**: SDK batches and exports to Collector
   ```
   POST http://localhost:4318/v1/metrics
   Content-Type: application/x-protobuf
   
   ResourceMetrics:
     - pool.objects.borrowed = 1
       Attributes: {pool.name: "CanonicalEvent", object.type: "*schema.Event"}
   ```

4. **Collector Processing**: Converts to Prometheus format
   ```
   pool_objects_borrowed{pool_name="CanonicalEvent",object_type="*schema.Event"} 1
   ```

5. **Prometheus Scrape**: Collects from Collector endpoint
   ```
   GET http://localhost:8889/metrics
   ```

6. **Grafana Query**: Fetches from Prometheus
   ```promql
   pool_objects_active{pool_name="CanonicalEvent"}
   ```

## Configuration Files

### Gateway (internal/pool/manager.go)
```go
meter := otel.Meter("pool")
objectsBorrowedCounter, _ = meter.Int64Counter("pool.objects.borrowed",
    metric.WithDescription("Number of objects borrowed from pools"),
    metric.WithUnit("{object}"))
```

### OpenTelemetry Collector (otel-collector-config.yaml)
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: meltica

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

### Prometheus (prometheus.yml)
```yaml
scrape_configs:
  - job_name: 'meltica'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8889']
```

### Grafana Dashboard (datasource config)
```json
{
  "datasource": {
    "type": "prometheus",
    "url": "http://localhost:9090",
    "access": "proxy"
  }
}
```

## Metric Types & Storage

### Counters (Cumulative)
```
pool.objects.borrowed → pool_objects_borrowed_total
pool.objects.returned → pool_objects_returned_total
```
- Monotonically increasing
- Use `rate()` or `increase()` to calculate rate
- Storage: ~2 bytes per data point

### Gauges (Instantaneous)
```
pool.objects.active → pool_objects_active
```
- Current value at scrape time
- Can go up or down
- Storage: ~2 bytes per data point

### Observable Gauges (Callback-based)
```
pool.capacity → pool_capacity
pool.available → pool_available
pool.utilization → pool_utilization
```
- Computed on-demand when scraped
- No additional memory in application
- Storage: ~2 bytes per data point

### Histograms (Distribution)
```
pool.borrow.duration → pool_borrow_duration_bucket
                     → pool_borrow_duration_sum
                     → pool_borrow_duration_count
```
- Multiple series per metric (buckets)
- Use `histogram_quantile()` for percentiles
- Storage: ~2 bytes × num_buckets per scrape

## Resource Usage

### Gateway Application
- **CPU**: <1% overhead for metrics
- **Memory**: ~100KB for metric state
- **Network**: ~5KB/s to OTLP collector

### OpenTelemetry Collector
- **CPU**: <5% with batching
- **Memory**: ~50MB buffer
- **Network**: ~5KB/s to Prometheus

### Prometheus Server
- **Disk**: ~1-2 bytes per sample × retention
  - 5 pools × 7 metrics × 4 samples/min = 140 samples/min
  - 30 days retention = ~6MB
- **Memory**: ~3 bytes per series in RAM
- **CPU**: <5% for queries

### Grafana
- **CPU**: <1% (only when viewing)
- **Memory**: ~100MB base
- **Network**: PromQL queries on demand

## Troubleshooting Data Flow

### Check each stage:

**1. Gateway → Collector**
```bash
# Check gateway is sending
tcpdump -i lo -A 'port 4318' | grep pool

# Check collector receiving
curl http://localhost:8888/metrics | grep otelcol_receiver_accepted_metric_points
```

**2. Collector → Prometheus**
```bash
# Check collector exporter
curl http://localhost:8889/metrics | grep pool

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="meltica")'
```

**3. Prometheus → Grafana**
```bash
# Test query directly
curl 'http://localhost:9090/api/v1/query?query=pool_utilization' | jq

# Check Grafana datasource
curl -H "Authorization: Bearer $GRAFANA_TOKEN" \
  http://localhost:3000/api/datasources | jq
```

## Performance Optimization

### Gateway
- Use observable gauges for computed metrics (no storage)
- Batch metric writes (automatic with OTLP SDK)
- Use exemplars for tracing correlation

### Collector
- Enable batching: `batch_size: 1024`
- Set memory limit: `limit_mib: 512`
- Use appropriate scrape intervals (15-60s)

### Prometheus
- Reduce retention if disk is concern: `--storage.tsdb.retention.time=15d`
- Use recording rules for expensive queries
- Enable compression: `--storage.tsdb.min-block-duration=2h`

### Grafana
- Use dashboard variables for filtering
- Set reasonable refresh intervals (30s-1m)
- Cache dashboard JSON

## Security Considerations

1. **OTLP Endpoint**: Use TLS for production
   ```yaml
   receivers:
     otlp:
       protocols:
         grpc:
           tls:
             cert_file: /certs/server.crt
             key_file: /certs/server.key
   ```

2. **Prometheus Scrape**: Use authentication
   ```yaml
   scrape_configs:
     - job_name: 'meltica'
       basic_auth:
         username: 'prometheus'
         password_file: /secrets/prometheus_password
   ```

3. **Grafana Access**: Use role-based access control
   - Viewer: Read-only dashboards
   - Editor: Can modify panels
   - Admin: Full access

## Next Steps

1. Follow [POOL_METRICS_QUICKSTART.md](./POOL_METRICS_QUICKSTART.md) to set up
2. Import [grafana-pool-dashboard.json](./grafana-pool-dashboard.json)
3. Configure alerts using [grafana-pool-dashboard.md](./grafana-pool-dashboard.md)
4. Monitor and tune based on your workload
