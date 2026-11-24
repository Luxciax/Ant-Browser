# Phase 3: templates, readiness and cross-context evidence

## Publishable templates

The built-in v1 package contains three operational templates:

- `balanced-workspace`
- `persistent-workspace`
- `constrained-runner`

Templates change session restoration and process memory policy only. They do
not contain identity seeds, commercial-browser samples, detector scores, GPU
labels, or generated fingerprint values.

Hard rules describe conditions that must remain enforced by the application:
kernel resolution, managed argument ownership, and fail-closed proxy behavior.
Soft rules are recommendations such as explicit runtime verification and
explicit locale-link policy.

## Launch readiness receipt

The v2 receipt now records the actual launch progression:

`prepared -> process-started -> debug-pending -> ready`

and may terminate as `failed` or `stopped`.

The receipt includes process/debug timestamps, PID/debug port, current attempt,
and maximum attempts. This is distinct from configuration state
(`configured/effective/pending-next-launch`).

## Cross-context evidence

The explicit runtime check now measures Window plus:

- same-origin iframe;
- DedicatedWorker;
- SharedWorker when supported.

Service Worker applicability is reported separately. A Service Worker result is
never synthesized: if a controlled worker probe endpoint is unavailable, the
context is recorded as applicable-but-not-executed (or not applicable).

Execution gaps produce `unverified`, while actual mismatches produce
`mismatch`. This preserves the distinction between execution, observation and
assertion.
