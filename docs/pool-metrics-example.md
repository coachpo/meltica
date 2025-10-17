# Pool Metrics Example

After the enhancements, all pool metrics now include both `pool.name` and `object.type` attributes, enabling granular monitoring per object type.

## Quick Links

- **Quick Start**: See [POOL_METRICS_QUICKSTART.md](./POOL_METRICS_QUICKSTART.md) to get up and running in 5 minutes
- **Full Grafana Guide**: See [grafana-pool-dashboard.md](./grafana-pool-dashboard.md) for detailed panel configurations
- **Import Dashboard**: Use [grafana-pool-dashboard.json](./grafana-pool-dashboard.json) to import ready-to-use dashboard

## Metric Dimensions

Each metric now has two dimensions:
- `pool.name`: The pool identifier (e.g., "CanonicalEvent", "WsFrame", "OrderRequest")
- `object.type`: The Go type name (e.g., "*schema.Event", "*schema.WsFrame", "*schema.OrderRequest")

## Available Metrics

### Counters
- `pool.objects.borrowed{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Total borrowed
- `pool.objects.returned{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Total returned

### Gauges
- `pool.objects.active{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Currently borrowed
- `pool.capacity{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Total capacity
- `pool.available{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Available now
- `pool.utilization{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Ratio (0.0-1.0)

### Histograms
- `pool.borrow.duration{pool.name="CanonicalEvent", object.type="*schema.Event"}` - Borrow latency (ms)

## Example Queries

### Monitor CanonicalEvent pool utilization
```promql
pool.utilization{pool.name="CanonicalEvent"}
```

### Find pools running out of capacity
```promql
pool.available < 10
```

### Borrow latency by type
```promql
histogram_quantile(0.99, pool.borrow.duration{object.type="*schema.Event"})
```

### Active objects by type across all pools
```promql
sum by (object.type) (pool.objects.active)
```

### Compare pool efficiency
```promql
pool.utilization > 0.8
```

## Gateway Pools

The gateway registers these pools with their types:

| Pool Name        | Object Type           | Capacity |
|------------------|-----------------------|----------|
| WsFrame          | *schema.WsFrame       | 200      |
| ProviderRaw      | *schema.ProviderRaw   | 200      |
| CanonicalEvent   | *schema.Event         | 1000     |
| OrderRequest     | *schema.OrderRequest  | 20       |
| ExecReport       | *schema.ExecReport    | 20       |

Each type can now be monitored independently!
