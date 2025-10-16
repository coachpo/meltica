# Meltica Telemetry Stack

This directory contains a complete observability stack for Meltica using OpenTelemetry, Jaeger, Prometheus, and Grafana.

## Quick Start

1. **Start the telemetry stack:**
   ```bash
   cd deployments/telemetry
   docker-compose up -d
   ```

2. **Configure Meltica:**
   ```bash
   export OTEL_ENABLED=true
   export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
   export OTEL_EXPORTER_OTLP_INSECURE=true
   export OTEL_SERVICE_NAME=meltica-gateway
   ```

3. **Start Meltica:**
   ```bash
   cd ../..
   ./bin/gateway --config=streaming.yml
   ```

4. **Access UIs:**
   - **Jaeger UI** (Traces): http://localhost:16686
   - **Prometheus** (Metrics): http://localhost:9090
   - **Grafana** (Dashboards): http://localhost:3000 (admin/admin)
   - **OTEL Collector** (Health): http://localhost:13133

## Services

### OpenTelemetry Collector
- **Ports:**
  - 4317: OTLP gRPC receiver
  - 4318: OTLP HTTP receiver (used by Meltica)
  - 8889: Prometheus metrics exporter
  - 13133: Health check endpoint

### Jaeger
- **Port:** 16686 (UI)
- **Purpose:** Distributed tracing visualization
- **Features:**
  - Trace search and filtering
  - Service dependency graph
  - Trace timeline visualization
  - Performance analysis

### Prometheus
- **Port:** 9090
- **Purpose:** Time-series metrics storage and querying
- **Features:**
  - Metrics browser
  - PromQL query language
  - Alert evaluation

### Grafana
- **Port:** 3000
- **Purpose:** Unified observability dashboard
- **Credentials:** admin/admin
- **Features:**
  - Pre-configured Prometheus datasource
  - Pre-configured Jaeger datasource
  - Create custom dashboards

## Useful Prometheus Queries

### Event Throughput
```promql
# Events published per second by type
rate(meltica_databus_events_published_total[1m])

# Events ingested by dispatcher
rate(meltica_dispatcher_events_ingested_total[1m])
```

### Error Rates
```promql
# Delivery errors per second
rate(meltica_databus_delivery_errors_total[1m])

# Dropped events per second
rate(meltica_dispatcher_events_dropped_total[1m])
```

### Latency Metrics
```promql
# 95th percentile event processing duration
histogram_quantile(0.95, rate(meltica_dispatcher_processing_duration_bucket[5m]))

# 99th percentile pool borrow duration
histogram_quantile(0.99, rate(meltica_pool_borrow_duration_bucket[5m]))
```

### Resource Utilization
```promql
# Active subscribers by event type
meltica_databus_subscribers

# Active pooled objects by pool name
meltica_pool_objects_active

# Currently buffered events
meltica_dispatcher_events_buffered
```

### Operational Metrics
```promql
# Duplicate event rate
rate(meltica_dispatcher_events_duplicate_total[1m])

# Fanout distribution (average subscribers per event)
avg(meltica_databus_fanout_size)
```

## Useful Jaeger Queries

### Find Slow Operations
1. Go to Jaeger UI
2. Select service: `meltica`
3. Set operation: `databus.Publish`
4. Set Min Duration: `1ms`
5. Click "Find Traces"

### Trace Event Flow
1. Search for operation: `dispatcher.process_event`
2. Look for spans:
   - `dispatcher.process_event` → `dispatcher.publish_batch` → `databus.Publish` → `databus.dispatch`
3. Analyze timing and attributes

### Debug Errors
1. Select "Tags" → "error" → "true"
2. Review failed operations
3. Check span events and logs for error details

## Creating Grafana Dashboards

1. Log into Grafana: http://localhost:3000
2. Click "+" → "Dashboard"
3. Add panels with queries:
   - **Event Throughput Panel:**
     - Query: `sum(rate(meltica_databus_events_published_total[1m])) by (event_type)`
     - Visualization: Time series
   
   - **Pool Utilization Panel:**
     - Query: `meltica_pool_objects_active`
     - Visualization: Gauge
   
   - **Processing Latency Panel:**
     - Query: `histogram_quantile(0.95, rate(meltica_dispatcher_processing_duration_bucket[5m]))`
     - Visualization: Time series

## Stopping the Stack

```bash
docker-compose down

# To also remove volumes (data will be lost):
docker-compose down -v
```

## Troubleshooting

### No data in Prometheus
1. Check OTEL Collector is running: `curl http://localhost:13133`
2. Check OTEL Collector metrics endpoint: `curl http://localhost:8889/metrics`
3. Check Prometheus targets: http://localhost:9090/targets

### No traces in Jaeger
1. Verify Meltica is sending traces: Check application logs for "telemetry initialized"
2. Check OTEL Collector logs: `docker logs meltica-otel-collector`
3. Verify OTLP endpoint is accessible from Meltica

### High resource usage
1. Reduce batch size in `otel-collector-config.yaml`
2. Increase batch timeout to reduce frequency
3. Adjust memory limiter if needed

## Production Considerations

For production deployments:

1. **Security:**
   - Enable TLS for OTLP endpoints
   - Use authentication for Grafana, Prometheus
   - Restrict network access

2. **Scaling:**
   - Deploy OTEL Collector as a daemonset/sidecar
   - Use remote write for Prometheus
   - Consider managed services (e.g., Grafana Cloud, Jaeger SaaS)

3. **Retention:**
   - Configure trace retention in Jaeger
   - Set Prometheus retention period
   - Use object storage for long-term traces

4. **Sampling:**
   - Reduce trace sampling rate in production
   - Use tail-based sampling for interesting traces only
   - Implement adaptive sampling

5. **High Availability:**
   - Deploy multiple OTEL Collectors
   - Use Prometheus federation
   - Run Jaeger with Elasticsearch/Cassandra backend
