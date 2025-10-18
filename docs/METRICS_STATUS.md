# Meltica Metrics - Actual Status

**Verified against running system:**
- OTEL Collector: http://localhost:8889/metrics
- Prometheus: http://localhost:9090

## ✅ Fully Working Metrics

### Orderbook Metrics
```
meltica_orderbook_buffer_size           - Updates buffered during cold start
meltica_orderbook_gap_detected          - Sequence gaps detected
meltica_orderbook_snapshot_applied      - Snapshots applied for recovery
meltica_orderbook_update_stale          - Stale updates rejected
meltica_orderbook_updates_replayed      - Buffered updates replayed
meltica_orderbook_coldstart_duration_*  - Time to first snapshot
```
**Dashboard**: meltica-event-type-deep-dive.json ✅ NOW FIXED

### Pool Metrics
```
meltica_pool_available                  - Available objects
meltica_pool_capacity                   - Total capacity
meltica_pool_objects_active             - Currently borrowed
meltica_pool_objects_borrowed           - Total borrowed (counter)
meltica_pool_borrow_duration_*          - Borrow latency histogram
```
**Dashboards**: meltica-pool-ops.json, meltica-resource-utilization.json ✅ WORKING

### Databus Metrics
```
meltica_databus_events_published        - Events published
meltica_databus_subscribers             - Active subscribers
meltica_databus_delivery_errors         - Delivery failures
meltica_databus_delivery_blocked        - Blocked deliveries (backpressure!)
meltica_databus_publish_duration_*      - Publish latency
meltica_databus_fanout_size_*           - Subscriber fanout size
```
**Note**: `meltica_databus_delivery_blocked` EXISTS! Not missing.

### Dispatcher Metrics
```
meltica_dispatcher_events_ingested      - Events received
meltica_dispatcher_events_dropped       - Events dropped
meltica_dispatcher_events_duplicate     - Duplicate events
meltica_dispatcher_events_buffered      - Currently buffered
meltica_dispatcher_processing_duration_* - Processing latency
meltica_dispatcher_routing_version      - Routing table version
```

### WebSocket Client Metrics
```
meltica_wsclient_frames_processed            - Frames processed
meltica_wsclient_frame_processing_duration_* - Frame processing latency
```

## ❌ Missing Metrics

### 1. Control Bus Metrics - **NOT EMITTED**
```
meltica_controlbus_send_duration  - ❌ NOT FOUND
meltica_controlbus_send_errors    - ❌ NOT FOUND
meltica_controlbus_queue_depth    - ❌ NOT FOUND
```

**Status**: Code exists in `internal/bus/controlbus/memory.go` but **no data is being emitted**.

**Possible Reasons**:
1. Control bus is not being used in current code paths
2. MemoryBus.Send() not being called
3. Meters not properly registered

**Dashboard Impact**: meltica-alerting-sli.json Panel 3 should use dispatcher metrics as proxy

**TODO**: Investigate why control bus metrics aren't appearing

### 2. Pool Caller Tracking - **NOT IMPLEMENTED**
```
meltica_pool_active_callers  - ❌ NOT IMPLEMENTED
```

**Status**: No instrumentation exists to track which callers hold pool objects

**Impact**: Cannot debug pool leaks by identifying problematic code paths

**Dashboard**: meltica-pool-ops.json Panel 4 shows "no data"

**To Implement**: Add caller tracking using runtime.Caller() in pool.Get()

## Summary

| Category | Metrics | Status | Dashboard TODO |
|----------|---------|--------|----------------|
| Orderbook | 6/6 | ✅ All working | ✅ Fixed |
| Pool | 5/6 | ✅ Mostly working | ⚠️ One panel empty |
| Databus | 6/6 | ✅ All working including blocked | ✅ Working |
| Dispatcher | 6/6 | ✅ All working | ✅ Working |
| WebSocket | 2/2 | ✅ Working | ✅ Working |
| Control Bus | 0/3 | ❌ None emitted | ❌ Use proxy metric |

## Dashboard Status After Fixes

### ✅ Resolved (2/3 TODOs)
1. **Orderbook Gap Monitoring** - Panel now shows real metrics ✅
2. **Databus Backpressure** - meltica_databus_delivery_blocked exists ✅

### ❌ Still Missing (1/3 TODOs)  
1. **Control Bus Metrics** - Need to investigate why not emitting
2. **Pool Caller Tracking** - Need to implement instrumentation

## Recommended Actions

### High Priority
1. **Fix control bus metrics** - Debug why they're not emitting
   - Check if MemoryBus.Send() is being called
   - Verify meter registration
   - Add logging to Send() method

### Medium Priority
2. **Add pool caller tracking** - Implement in pool manager
   - Use runtime.Caller(2) to get calling function
   - Add meltica_pool_active_callers UpDownCounter
   - Track per pool_name and caller labels

### Low Priority
3. **Update dashboards** - Revert control bus panel to dispatcher proxy
