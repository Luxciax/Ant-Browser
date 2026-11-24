# Script and workflow compatibility

Ant Browser already exposes saved automation scripts and workflows through LaunchServer. OpenClaw should invoke those public operations rather than reading script files or recreating the local runner.

## Execution ownership

Ant Browser owns:
- script/workflow lookup and persisted defaults;
- target selection and runtime preparation;
- local runner/CDP integration;
- run records, errors and artifacts;
- workflow pause/interruption/resume semantics.

OpenClaw owns:
- user intent and task orchestration;
- choosing a saved script/workflow and explicit overrides;
- deciding whether a retry or broader action is authorized.

## Scripts

Use:
- `GET /api/automation/scripts` to discover saved scripts;
- `GET /api/automation/scripts/{id}` for one script;
- `POST /api/automation/scripts/run` to execute;
- `GET /api/automation/scripts/runs` for history.

Pass only user-authorized selector/parameter overrides. If the saved script already owns its target/defaults, prefer the minimal run request rather than reconstructing stored configuration.

## Workflows

Use the `/api/automation/workflows` and `/api/automation/workflow-runs` route groups registered by LaunchServer. Workflow execution supports persisted run state; resume the stored run instead of rebuilding a partially completed workflow externally.

## Browser takeover

When OpenClaw needs interactive browser control rather than a saved automation, use `POST /api/runtime/session` and attach only after the response reports a ready target with a usable CDP URL.

## Compatibility rule

Keep external JSON typed at the HTTP boundary and adapt to internal storage/service types in one place. Do not add an OpenClaw-only copy of selector, params, retry, artifact, or CDP logic.
