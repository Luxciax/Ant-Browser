# LaunchServer public API contract

This reference summarizes route groups registered by current source. `backend/internal/launchcode/server_http.go` is authoritative for routes; `backend/internal/launchcode/auth.go` is authoritative for authorization.

## Control and diagnostics

| Route | Purpose |
| --- | --- |
| `GET /api/health` | service health |
| `/api/diagnostics` | control-plane diagnostics |
| `/api/sync/status` | backup-channel sync status |
| `/api/network/observe` | create a host network observation |
| `/api/network/observations/{id}` | read a stored observation |
| `/api/system/integration` | optional host integration status/actions |

## Environment lifecycle

| Route | Purpose |
| --- | --- |
| `/api/profiles` | profile collection CRUD |
| `/api/profiles/{id}/...` | profile detail/runtime/evidence/capability actions |
| `GET /api/runtime/active` | current unified CDP target |
| `POST /api/runtime/session` | resolve/start a target and wait for CDP readiness |
| `POST /api/runtime/status` | selector-based runtime status |
| `POST /api/runtime/stop` | selector-based stop |
| `/api/launch` and `/api/launch/{code}` | launch operations |
| `/api/launch/logs` | launch logs |
| `/api/launch/tasks` | tracked launch tasks |
| `/api/batch/...` | batch operations |
| `/api/trash...` | recycle-bin operations |

## Automation

| Route | Purpose |
| --- | --- |
| `/api/automation/scripts` | saved script collection |
| `/api/automation/scripts/{id}` | saved script resource |
| `POST /api/automation/scripts/run` | run a saved script |
| `/api/automation/scripts/runs` | script run history |
| `/api/automation/hooks/{...}` | configured public automation hooks |
| `/api/automation/workflows` | workflow collection |
| `/api/automation/workflows/{id}` | workflow resource |
| `POST /api/automation/workflows/run` | run a workflow |
| `/api/automation/workflow-runs` | workflow run history/resume |

## MCP and CDP

- MCP is mounted by `mcpMount()` when enabled.
- Unmatched paths fall through to the CDP reverse proxy.
- `/api/*`, `/mcp`, `/json*`, and `/devtools*` participate in API authentication when enabled.

## Authorization scopes

- `environment-read`
- `page-read`
- `page-action`
- `script`
- `environment-admin`
- `backup-credential-export`
- `*`

Route/method-to-scope mapping is code. Re-read `requiredScopeForRequest` before changing a client contract.

## Runtime session

`POST /api/runtime/session` is the preferred endpoint when a client needs a usable CDP target. Do not attach until the actual response reports a ready target and non-empty CDP URL. Runtime status/stop are single-target operations; ambiguous selectors must not be resolved silently.
