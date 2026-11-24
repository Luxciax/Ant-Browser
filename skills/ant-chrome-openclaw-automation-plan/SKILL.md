---
name: ant-chrome-openclaw-automation-plan
description: Implement, review, or audit Ant Browser and OpenClaw integration code and public LaunchServer contracts. Use for changes to runtime/CDP session control, automation scripts or workflows, API authorization, compatibility mapping, and integration documentation; use the sibling ant-chrome-openclaw skill for day-to-day invocation.
---

# Ant Browser OpenClaw Integration

Keep OpenClaw as an orchestration client and Ant Browser as the execution/runtime owner.

## Read only what is needed

- `references/overview.md`: architecture boundary.
- `references/api-contract.md`: current LaunchServer route groups and authorization source.
- `references/script-compat.md`: script/CDP compatibility details.

## Engineering rules

1. Inspect `backend/internal/launchcode/server_http.go` before changing or documenting an HTTP route.
2. Inspect `backend/internal/launchcode/auth.go` before changing authorization or scopes.
3. Reuse existing runtime, automation script, workflow, batch, profile, and CDP services. Do not create OpenClaw-only execution stacks.
4. Keep external JSON objects typed at the boundary; convert only where the existing internal model requires it.
5. Prefer a thin compatibility adapter over duplicate business logic.
6. Update the sibling operational Skill and API reference in the same change when the public contract changes.

## Validation

Run focused tests for the touched LaunchServer package, then repository gates appropriate to the change. For substantial integration changes:

```text
node tools/check-repo-consistency.mjs
go test -p=1 ./backend/... ./internal/... ./tools/...
go vet ./backend/... ./internal/... ./tools/...
go test -run ^$ -p=1 ./...
```

Do not use machine-specific validator paths in repository instructions.
