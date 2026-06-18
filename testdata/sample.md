# Agent Diagnostic Report

## Environment Summary

The workspace was scanned across three compute nodes. All services responded
within normal latency bounds except the vector store, which timed out twice
during the initial handshake phase. The issue was traced to a misconfigured
connection pool with **too few idle connections**, causing requests to queue
behind a _slow keepalive sweep_. The fix is a one-liner: set `max_idle=32` in
the pool config. See [upstream docs](https://example.com/pool-config) for
details. Legacy clients using ~~keep-alive v1~~ should migrate immediately.

## Findings

### Dependency Tree

- Core services
    - auth-gateway (healthy)
    - token-validator (healthy)
    - rate-limiter (degraded)
        - upstream quota exceeded
        - retry-after: 30 s
- Storage layer
    - postgres-primary (healthy)
    - postgres-replica (lagging 42 s)
    - vector-store (timeout — see above)

### Reproduction Steps

1. Start the worker process with `POOL_SIZE=2`
2. Fire 50 concurrent requests to `/api/embed`
3. Observe connection exhaustion after ~8 s
4. Tail the log: `tail -f /var/log/worker.log | grep POOL`

---

## Code Snapshot

The offending initialisation (Python):

```python
import asyncpg

async def build_pool(dsn: str):
    # BUG: max_size too small under load; idle connections recycled before reuse
    pool = await asyncpg.create_pool(dsn, min_size=1, max_size=2, max_inactive_connection_lifetime=10.0)
    return pool
```

Fixed version — note the very long comment explaining the choice of 32:

```python
async def build_pool(dsn: str):
    pool = await asyncpg.create_pool(dsn, min_size=4, max_size=32, max_inactive_connection_lifetime=300.0)  # 32 keeps p99 latency under 50 ms at 200 rps on a 4-core node; tune down if RAM is tight
    return pool
```

---

## Latency Breakdown

| Service          | p50 (ms) | p95 (ms) | p99 (ms) | Error rate |
|------------------|----------|----------|----------|------------|
| auth-gateway     | 4        | 11       | 18       | 0.00 %     |
| token-validator  | 6        | 14       | 22       | 0.00 %     |
| rate-limiter     | 12       | 340      | 980      | 1.40 %     |
| postgres-primary | 2        | 8        | 15       | 0.00 %     |
| vector-store     | —        | —        | —        | 100.00 %   |

---

## Recommendations

> This is a general observation about the overall system health that spans
> multiple sentences and is long enough to wrap at a narrow terminal width —
> the admonition border should stay flush on every wrapped line with no
> overflow beyond the pane edge.

> [!note]
> The connection pool exhaustion is the **root cause** of the vector-store
> timeouts. Fixing `max_size` will resolve both symptoms in one deployment.

> [!tip]
> Run `pgbouncer` in front of the primary if you need more than 64 concurrent
> connections — asyncpg pools are per-process and do not coordinate across
> workers.

> [!important]
> The replica lag of 42 seconds exceeds the SLA threshold of 10 seconds.
> Promote a new replica from the primary before the next release window.

> [!warning]
> Do not restart the rate-limiter pod until the quota resets at 00:00 UTC;
> an unclean shutdown during the reset window corrupts the sliding window
> counter stored in Redis.

> [!caution]
> Dropping and recreating the vector-store index will delete all embeddings
> permanently. Ensure a snapshot backup exists before running any migration
> against the production index.
