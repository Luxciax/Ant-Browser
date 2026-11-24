# Phase 4: bounded batch operations

Batch create/start/stop operations return an operation record with:

- operation ID;
- kind and terminal status;
- requested concurrency;
- total/succeeded/failed counts;
- per-item profile/client key;
- per-item runtime snapshot or error.

The public concurrency range is capped at 8 and each request is capped at 100
items. Existing single-profile lifecycle and maintenance locks remain
authoritative. The batch scheduler therefore never bypasses file transactions,
proxy bridge ownership, or per-profile lifecycle serialization just to create
unsafe parallelism.

Operations are stored in application runtime state and can be queried by
operation ID after a partial failure. Per-item failures include a best-effort
current runtime snapshot so the caller can safely decide whether to retry,
stop, or inspect the affected profile.

Batch operations are best-effort, not an all-or-nothing transaction across
independent browser processes.

## Restart recovery

Operation records are persisted to `data/batch_operations.v1.json` with
restricted file permissions and a bounded history of 200 records. Runtime
profile snapshots are sanitized before persistence (proxy configuration and
effective launch arguments are omitted).

If Ant Browser restarts while an operation is still marked `running`, the
record is recovered as `interrupted` with a finish timestamp. The operation
ID and completed per-item results remain queryable, allowing the caller to
inspect actual profile runtime state before selectively retrying failed or
unfinished items.
