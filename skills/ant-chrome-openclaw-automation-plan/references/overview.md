# Integration boundary

OpenClaw is an orchestration client. Ant Browser owns environments, runtime state, LaunchServer, CDP routing, saved automation and workflow execution.

## Preferred paths

- Browser attachment: `POST /api/runtime/session`.
- Runtime inspection/stop: `POST /api/runtime/status` and `POST /api/runtime/stop`.
- Saved scripts: `/api/automation/scripts` and `/api/automation/scripts/run`.
- Workflows/RPA: `/api/automation/workflows` and `/api/automation/workflows/run`.
- Recent failures: inspect launch/script/workflow run records before retrying.

Do not create an OpenClaw-specific execution runtime when the same operation already exists in Ant Browser. If an external client eventually requires fixed legacy route names, add only a thin compatibility mapping to the existing service layer.

## Authority

- Routes: `backend/internal/launchcode/server_http.go`
- HTTP scopes: `backend/internal/launchcode/auth.go`
- Automation execution: current backend automation services
- Exact request examples: `api-contract.md` and the operational sibling Skill references

Do not maintain phase-status lists here; use source and tests for current delivery state.
