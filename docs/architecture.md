# Architecture

This document describes durable repository structure. Source code remains authoritative.

## Environment and launch

- Profiles remain the persisted browser records. EnvironmentView projects lifecycle identity, persona/configuration summaries, kernel binding, network policy and runtime state without exposing credentials.
- Logical environment identity is separate from the database profile ID.
- Browser start paths converge on the existing launch preparation/execution service. LaunchPlan/receipt data describes configured, prepared, effective and pending-next-launch state instead of duplicating launch compilation.
- Kernel capability state distinguishes configurable/emitted/acknowledged/observed/verified evidence. Accepting a setting or matching a browser version is not verification.

## Lifecycle

- Rename/tag/group edits do not refresh environment identity.
- Clone, identity-preserving migration, explicit identity refresh, kernel switch and storage migration remain distinct operations.
- Runtime receipts/evidence are revision-bound and must not be reused as proof after an incompatible environment/kernel revision.
- Delete uses the existing recycle/transaction path; permanent deletion remains explicit.

## Network

- NetworkPolicy is scoped to Ant Browser and does not silently rewrite host DNS, firewall, TUN or other proxy applications.
- A configured proxy follows fail-closed behavior unless the caller explicitly selects a supported alternative.
- Xray+sing-box and Mihomo are separate connector stacks. Do not silently mix them.
- Locale/link recommendations are explicit policy; proxy geography is not an implicit identity rewrite.

## Automation and batch operations

- Saved automation scripts and Workflow/RPA reuse existing browser/page runtime services.
- Workflow state persists run/attempt information needed for pause, interruption and resume.
- Side-effecting retries require explicit permission.
- Batch operations are bounded best-effort operations over existing per-profile locks; they are not cross-process transactions.

## Frontend

- Fish UI under `frontend/src/ui-v2/fish` is the active presentation layer.
- Domain APIs, types, hooks and helpers stay under `frontend/src/modules` and may be reused by Fish pages.
- Historical routes are compatibility redirects to the current page implementation; do not maintain parallel legacy pages.

## Templates

- Built-in environment templates describe supported configuration/lifecycle defaults and compatibility requirements.
- Cross-OS/mobile compatibility remains `requires-compatible-kernel` until a matching runtime is actually verified.
- Templates do not import commercial identity material or turn external research samples into browser identity data.
