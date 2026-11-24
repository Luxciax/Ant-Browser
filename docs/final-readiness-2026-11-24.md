# Final readiness report — 2026-11-24

This document records the final repository-side validation completed after phases T01–T20.

## Candidate validated

The finalization chain was validated on Windows in an isolated detached worktree. The user's pre-existing local Ant-Browser worktree was left untouched because its index/worktree state was not clean.

## Windows validation completed

The following commands completed successfully against the finalization code line:

- `go test -p=1 ./backend/... ./internal/... ./tools/...`
- `go vet ./backend/... ./internal/... ./tools/...`
- `go test -run ^$ -p=1 ./...`
- `npm ci`
- `npm run build`
- `npm audit --json`
- `go run github.com/wailsapp/wails/v2/cmd/wails@v2.17.1 build -clean`

Results:

- backend/internal/tools Go tests: PASS (serial package execution to avoid Windows resource-sensitive test contention);
- Go vet: PASS;
- full Go module compile gate: PASS;
- frontend TypeScript/Vite production build: PASS;
- Wails v2.17.1 full Windows production build: PASS (`build/bin/ant-chrome.exe` generated);
- final full npm audit: 0 known vulnerabilities;
- final dependency tree: no invalid peer/dependency entries in the audited Tailwind/Vite chains.

## Wails / Go toolchain migration

The desktop application was upgraded from Wails v2.12.0 to Wails v2.17.1. The project Go directive is now 1.26.0. This resolves the old Wails package-analysis failure seen with the newer Go toolchain and was verified by a full Windows production build, including bindings generation, frontend compilation, application assets and final executable compilation.

## Frontend dependency migration

The validated frontend line uses:

- React Router DOM 7.18.6;
- js-yaml 4.3.2;
- Vite 8.3.8;
- @vitejs/plugin-react-swc 4.3.3;
- PostCSS 8.5.33;
- Autoprefixer 10.7.0.

Scoped npm overrides keep the Tailwind 3 development dependency tree on audited versions without overriding Vite's own picomatch tree.

Vite 8 / Rolldown requires `manualChunks` to be a function, so the existing React vendor chunk rule was migrated to an equivalent function form.

## UI closure

Fish UI now exposes the previously backend-only phase 3–5 surfaces:

- EnvironmentView v2 and template compatibility;
- NetworkPolicy;
- workflow/RPA editor, scheduling, run history and resume;
- API/MCP capability scopes and key rotation/revocation;
- host network observations;
- backup-channel sync status;
- recycle bin;
- optional `antbrowser://` Windows protocol integration.

## Reproducible manual quality gate

`.github/workflows/quality.yml` is manual-only (`workflow_dispatch`). It runs the same core Go test/vet/compile and frontend install/audit/build gates without publishing artifacts or creating a release.

## Intentionally unresolved external-infrastructure boundaries

These are not represented as verified features until corresponding infrastructure exists:

- on-wire/server-side JA3/JA4 capture;
- independent QUIC / HTTP/3 packet-level proof;
- browser-route and server-seen telemetry from a controlled observation endpoint;
- continuous real-time bidirectional multi-machine profile sync;
- actual Android/iOS/macOS runtime compatibility where a compatible kernel/runtime is not present.

Ant keeps these states explicit as unavailable/not-observed/requires-compatible-kernel rather than synthesizing evidence.
