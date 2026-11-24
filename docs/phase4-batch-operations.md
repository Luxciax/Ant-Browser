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
