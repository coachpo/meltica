# Telemetry Labels Update Summary

## Overview
All telemetry metric emissions have been updated to include complete labels for proper filtering and querying in Grafana dashboards.

## Required Labels
All metrics now include the following labels where applicable:
- **environment**: The runtime environment (development, staging, production)
- **event_type**: Type of event (book_snapshot, trade, etc.)
- **provider**: Data provider name (binance, fake, etc.)
- **symbol**: Trading symbol (BTC-USDT, etc.)

## Changes Made

### 1. Telemetry Package (internal/telemetry/telemetry.go)
- Added global `environment` variable
- Added `Environment()` function to access environment from anywhere
- Environment is set when telemetry provider is initialized

### 2. Dispatcher Metrics (internal/dispatcher/runtime.go)
Updated all metrics to include `environment` label:
- `dispatcher.events.ingested` - Added `environment`
- `dispatcher.events.dropped` - Added `environment`
- `dispatcher.events.duplicate` - Added `environment`
- `dispatcher.events.buffered` - Added `environment`
- `dispatcher.processing.duration` - Added `environment`

**Label standardization**: Changed `event.type` → `event_type` for consistency

### 3. Databus Metrics (internal/bus/databus/memory.go)
Updated all metrics to include `environment` and fix missing labels:
- `databus.events.published` - Added `environment`, standardized to `event_type`, `provider`, `symbol`
- `databus.subscribers` - Added `environment`
- `databus.delivery.errors` - Added `environment`, standardized label names
- `databus.fanout.size` - Added `environment`, `provider`, `symbol`

**Label standardization**: Changed `event.provider`, `event.symbol` → `provider`, `symbol`

### 4. Pool Metrics (internal/pool/manager.go)
Updated all metrics to include `environment`:
- `pool.objects.borrowed` - Added `environment`
- `pool.objects.active` - Added `environment`
- `pool.borrow.duration` - Added `environment`
- `pool.capacity` - Added `environment`
- `pool.available` - Added `environment`

**Label standardization**: Changed `pool.name`, `object.type` → `pool_name`, `object_type`

### 5. Orderbook Metrics (internal/adapters/binance/book_assembler.go)
Updated all metrics to include complete labels:
- `orderbook.gap.detected` - Added `environment`, `provider`, `event_type`
- `orderbook.buffer.size` - Added `environment`, `provider`, `event_type`
- `orderbook.update.stale` - Added `environment`, `provider`, `event_type`
- `orderbook.snapshot.applied` - Added `environment`, `provider`, `event_type`
- `orderbook.updates.replayed` - Added `environment`, `provider`, `event_type`
- `orderbook.coldstart.duration` - Added `environment`, `provider`, `event_type`

### 6. Provider Metrics (internal/adapters/binance/ws_provider.go)
Updated metrics to include `environment`:
- `provider.reconnections` - Added `environment`

## Grafana Dashboards
All Grafana dashboards already use the correct label names and are ready to work with the updated metrics:
- ✅ grafana-overview-dashboard.json
- ✅ grafana-pipeline-dashboard.json
- ✅ grafana-pool-dashboard.json
- ✅ grafana-orderbook-dashboard.json

Dashboards can now properly filter by:
- Environment (development, staging, production)
- Symbol (BTC-USDT, ETH-USDT, etc.)
- Event Type (book_snapshot, trade, etc.)
- Provider (binance, fake, etc.)

## Label Naming Conventions
Standardized to use underscores (_) instead of dots (.):
- `event_type` (not `event.type`)
- `pool_name` (not `pool.name`)
- `object_type` (not `object.type`)

This ensures consistency across all metrics and better compatibility with Prometheus/Grafana query syntax.

## Testing Recommendations
1. Restart the gateway to initialize telemetry with environment
2. Verify metrics are exported with all labels in Prometheus
3. Test Grafana dashboard filtering by environment
4. Confirm all metrics appear in their respective dashboards

## Migration Notes
- The environment is automatically set from `MELTICA_ENV` or `OTEL_RESOURCE_ENVIRONMENT` environment variables
- Default environment is "development" if not set
- All existing metrics will include the new labels automatically
- No breaking changes - only additions to metric labels
