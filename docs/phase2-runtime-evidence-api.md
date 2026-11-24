# Phase 2 runtime evidence and control API

Phase 2 distinguishes configuration, emission, observation, and verification.
Ordinary status reads never execute the runtime probe.

## Explicit runtime verification

The existing basic runtime self-check is reused as an explicit evidence source.
It records status only; it does not turn commercial research data into browser
configuration.

Evidence fields keep these concepts separate:

- execution status: whether the check executed;
- observation status: whether runtime data was actually observed;
- assertion status: whether configured expectations matched observed values.

Language, timezone, hardware concurrency and platform can become `verified`
only when a non-empty configured expectation exists and the observed value
matches. Canvas, Audio and ClientRects hashes are observation-only because the
current contract has no expected hash. WebGL values are also observation-only
unless a supported expected value exists.

The existing Window/DedicatedWorker consistency check is represented separately.
No Service Worker or cross-origin frame claim is made by this phase.

## HTTP

Authenticated local HTTP exposes:

- `GET /api/profiles/{id}/environment`
- `GET /api/profiles/{id}/launch-state`
- `GET /api/profiles/{id}/launch-receipt`
- `GET /api/profiles/{id}/capabilities`
- `GET /api/profiles/{id}/runtime-evidence`
- `POST /api/profiles/{id}/runtime-verify`

Only the final endpoint executes the runtime probe.

## MCP

The corresponding tools are:

- `ant_instance_environment`
- `ant_instance_launch_state`
- `ant_instance_launch_receipt`
- `ant_instance_capabilities`
- `ant_runtime_evidence`
- `ant_runtime_verify`

The first five are passive reads. `ant_runtime_verify` is an explicit runtime
measurement operation.

## Capability promotion

The kernel manifest starts from configuration/compiler evidence. A capability
can be promoted from `emitted` to `observed` or `verified` only when the
latest runtime evidence belongs to the current environment revision.

No detector score, third-party anti-fraud result, or browser-version guess is
used as a release gate or verification source.
