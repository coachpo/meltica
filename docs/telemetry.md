# Telemetry in Meltica

Meltica uses OpenTelemetry for distributed tracing and metrics collection. This provides comprehensive observability into the application's behavior, performance, and resource usage.

## Configuration

Telemetry is configured via environment variables:

### Core Settings

- `OTEL_ENABLED` - Enable/disable telemetry (default: `true`, set to `false` to disable)
- `OTEL_SERVICE_NAME` - Service name for telemetry (default: `meltica`)
- `OTEL_SERVICE_NAMESPACE` - Service namespace (optional)
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP endpoint for traces and metrics (default: `localhost:4318`)
- `OTEL_EXPORTER_OTLP_INSECURE` - Use insecure connection (default: `false`, set to `true` for local development)
- `OTEL_CONSOLE_EXPORTER` - Enable console exporter for debugging (default: `false`)

### Example Configuration

```bash
# Development setup with local OTLP collector
export OTEL_ENABLED=true
export OTEL_SERVICE_NAME=meltica-gateway
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
export OTEL_EXPORTER_OTLP_INSECURE=true

# Production setup
export OTEL_ENABLED=true
export OTEL_SERVICE_NAME=meltica-gateway
export OTEL_SERVICE_NAMESPACE=production
export OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.example.com:4318

# Disable telemetry
export OTEL_ENABLED=false
```

## Instrumented Components

### 1. Data Bus (`internal/bus/databus`)

**Traces:**
- `databus.Publish` - Event publication with fanout
- `databus.dispatch` - Event delivery to subscribers

**Metrics:**
- `databus.events.published` - Counter of published events by type and provider
- `databus.subscribers` - Gauge of active subscribers by event type
- `databus.delivery.errors` - Counter of delivery errors
- `databus.fanout.size` - Histogram of subscriber counts per fanout

**Attributes:**
- `event.type` - Canonical event type
- `event.provider` - Provider name (e.g., binance, fake)
- `error` - Error type (clone_batch_failed, dispatch_failed)

### 2. Dispatcher Runtime (`internal/dispatcher`)

**Traces:**
- `dispatcher.process_event` - Individual event processing
- `dispatcher.publish_batch` - Batch publication to data bus

**Metrics:**
- `dispatcher.events.ingested` - Counter of events ingested
- `dispatcher.events.dropped` - Counter of dropped events
- `dispatcher.events.duplicate` - Counter of duplicate events detected
- `dispatcher.events.buffered` - Gauge of currently buffered events
- `dispatcher.processing.duration` - Histogram of event processing time (ms)

**Attributes:**
- `event.type` - Canonical event type
- `event.provider` - Provider name
- `event.id` - Event identifier
- `reason` - Drop reason (not_buffered)

### 3. Pool Manager (`internal/pool`)

**Traces:**
- `pool.Get` - Object borrowing from pool

**Metrics:**
- `pool.objects.borrowed` - Counter of borrowed objects
- `pool.objects.returned` - Counter of returned objects
- `pool.objects.active` - Gauge of currently active borrowed objects
- `pool.borrow.duration` - Histogram of borrow operation time (ms)

**Attributes:**
- `pool.name` - Pool name (CanonicalEvent, WsFrame, ProviderRaw, etc.)

## Setting up an OTLP Collector

### Local Development with Docker

```bash
# Run OpenTelemetry Collector
docker run -d --name otel-collector \
  -p 4317:4317 \
  -p 4318:4318 \
  otel/opentelemetry-collector:latest

# Configure Meltica
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
export OTEL_EXPORTER_OTLP_INSECURE=true
```

### With Jaeger for Traces

```bash
# Run Jaeger all-in-one
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Access Jaeger UI at http://localhost:16686
```

### With Prometheus for Metrics

Create an OpenTelemetry Collector config (`otel-collector-config.yaml`):

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger]
    metrics:
      receivers: [otlp]
      exporters: [prometheus]
```

Run with Docker Compose:

```yaml
version: '3'
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14250:14250"

  otel-collector:
    image: otel/opentelemetry-collector:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
    ports:
      - "4317:4317"
      - "4318:4318"
      - "8889:8889"
    depends_on:
      - jaeger

  prometheus:
    image: prom/prometheus:latest
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
    depends_on:
      - otel-collector
```

## Viewing Telemetry Data

### Traces

1. Access Jaeger UI at http://localhost:16686
2. Select service: `meltica`
3. Search for traces

Key trace operations to look for:
- `databus.Publish` - Shows event publication flow
- `dispatcher.process_event` - Shows event processing
- `pool.Get` - Shows pool resource acquisition

### Metrics

1. Access Prometheus at http://localhost:9090
2. Query metrics:

```promql
# Event throughput
rate(databus_events_published_total[1m])

# Active subscribers
databus_subscribers

# Event processing duration (95th percentile)
histogram_quantile(0.95, rate(dispatcher_processing_duration_bucket[5m]))

# Pool utilization
pool_objects_active

# Error rate
rate(databus_delivery_errors_total[1m])
```

## Performance Impact

Telemetry has minimal performance impact:

- **Tracing**: ~1-5µs per span creation
- **Metrics**: ~100-500ns per counter increment
- **Memory**: ~10-20MB for SDK buffers

Sampling can be adjusted via the `SampleRate` config field (currently set to 1.0 = 100%).

## Troubleshooting

### No data appearing in collector

1. Check telemetry is enabled:
   ```bash
   echo $OTEL_ENABLED
   ```

2. Verify endpoint is reachable:
   ```bash
   curl http://localhost:4318/v1/traces
   ```

3. Check application logs for telemetry initialization:
   ```
   gateway telemetry initialized: endpoint=localhost:4318
   ```

### High memory usage

Reduce metric collection interval in code:
```go
telemetryCfg.MetricInterval = 60 * time.Second  // Increase from 30s
```

### Missing traces

Ensure context is properly propagated through the call chain. All instrumented functions accept `context.Context` as the first parameter.

## Future Enhancements

Potential additional instrumentation points:

1. **Control Bus** (`internal/bus/controlbus`)
   - Command send/acknowledgement latency
   - Consumer queue depth

2. **Snapshot Store** (`internal/snapshot`)
   - Snapshot read/write operations
   - CAS retry counts
   - Cache hit rates

3. **HTTP Handlers**
   - Request duration
   - Response codes
   - Request body sizes

4. **Provider Adapters**
   - WebSocket connection lifecycle
   - Message receive rates
   - Reconnection attempts
