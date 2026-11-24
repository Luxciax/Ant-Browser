---
name: ant-chrome-openclaw
description: Control a same-host Ant Browser instance through LaunchServer and remote CDP. Use for Ant Browser health checks, profile selection, launch/runtime control, saved automation scripts or workflows, diagnostics, and other public LaunchServer operations from OpenClaw.
---

# Ant Browser via OpenClaw

Use Ant Browser as the source of truth for environments, launch codes, proxies, cores, runtime state, and saved automation.

Read `references/setup.md` only for first-time setup or connection failures. Read `references/http.md` when exact HTTP request examples are needed.

## Defaults

- Base URL: `ANT_CHROME_BASE_URL`, default `http://127.0.0.1:19876`
- API header: `ANT_CHROME_API_HEADER`, default `X-Ant-Api-Key`
- API key: `ANT_CHROME_API_KEY`, when authentication is enabled

## Workflow

1. Check `GET /api/health` before a mutating operation.
2. Use the narrowest identifier available: launch code, profile ID, profile name, then stronger selectors.
3. Use `POST /api/runtime/session` when the goal is to prepare a browser for CDP attachment. Require `ready=true` and a non-empty `cdpUrl` before attaching.
4. Use `POST /api/runtime/status` for selector-based state checks and `POST /api/runtime/stop` to stop the Ant Browser instance.
5. Prefer saved automation endpoints over reconstructing existing scripts or workflows: `/api/automation/scripts` and `/api/automation/workflows`.
6. Query `/api/launch/logs` or run records before retrying a failed operation.
7. Treat authentication scope failures as authorization failures; do not retry with broader behavior unless the user authorizes it.

## Rules

- Do not invent routes. The authoritative route registry is `backend/internal/launchcode/server_http.go`; scope rules are in `backend/internal/launchcode/auth.go`.
- Do not claim that detaching an external CDP client stops Ant Browser.
- Do not silently choose among ambiguous profile matches.
- Do not expose API keys, proxy credentials, cookies, or other stored secrets in prompts or logs.
- Keep missing runtime/network evidence as unavailable rather than inferring it.
