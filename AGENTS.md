# Ant Browser Engineering Rules

## Source of truth

- Treat current source, schemas, tests, generated bindings, and successful build output as authoritative.
- Treat README, docs, skills, comments, and historical audit files as secondary. Update them in the same change when behavior changes.
- Do not add placeholders, fake buttons, unconnected APIs, or UI states that claim unsupported capability.

## Structure

- The active frontend is the Fish UI under `frontend/src/ui-v2/fish`.
- Keep one page implementation per user-facing route. Historical URLs may redirect to current routes; do not maintain parallel legacy pages.
- Reuse browser domain logic from `frontend/src/modules/browser` rather than duplicating it in page components.
- LaunchServer HTTP routing and authorization are defined under `backend/internal/launchcode`; never invent routes from docs.
- Proxy behavior is defined by the current proxy/runtime code. Preserve the explicit Xray+sing-box stack versus Mihomo stack boundary.

## State and security

- Prefer explicit persisted state and revisioned/runtime receipts over derived hidden state.
- Secrets must use SecretRef-backed storage. Do not introduce plaintext fallback for credentials.
- Missing runtime evidence stays unavailable/not-observed. Do not synthesize support, network evidence, or compatibility status.
- Keep destructive operations recoverable where the product already provides trash, transaction, or backup semantics.

## Verification

Run the smallest checks that cover the change, then the repository gates before merging substantial changes:

```text
node tools/check-repo-consistency.mjs
go test -p=1 ./backend/... ./internal/... ./tools/...
go vet ./backend/... ./internal/... ./tools/...
go test -run ^$ -p=1 ./...
npm --prefix frontend ci
npm --prefix frontend audit --audit-level=high
npm --prefix frontend run build
```

Windows backend tests use `-p=1` because resource-sensitive process/proxy tests have shown contention when packages run concurrently.

For release changes, derive Go from `go.mod`, derive the Wails CLI version from the Wails module in `go.mod`, and keep `wails.json`, `frontend/package.json`, and `frontend/package-lock.json` on the same product version.

## Repository hygiene

- Do not commit machine-specific absolute paths, local data, credentials, build caches, generated release outputs, or user profiles.
- Keep comments for non-obvious design intent and constraints; remove comments that merely restate code.
- Delete dead entry points and stale compatibility layers when their consumers are gone. Keep compatibility only at a narrow boundary such as a redirect or migration.
- Keep historical investigation material clearly separated from current architecture guidance.
