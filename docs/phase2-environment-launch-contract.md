# Phase 2 environment and launch contract

Phase 2 introduces a compatibility layer instead of replacing the existing
profile table in one destructive migration.

## Environment contract v2

Each legacy profile can be projected into a v2 environment DTO with:

- a stable environment ID stored separately from the profile/database ID;
- persona metadata (name, tags and keywords);
- configured/effective kernel binding;
- network binding summary without proxy credentials;
- browser storage/session settings;
- current runtime state;
- counts showing that legacy fingerprint/launch settings still exist.

The environment ID is lifecycle metadata. It is not a fingerprint seed and is
not derived from commercial research samples.

Migration 19 creates the environment identity table. Old profiles receive an
identity lazily when the v2 contract is requested. Profile-package database
snapshots carry this mapping so migration/restore can retain the logical
environment identity even when the destination profile ID changes.

## Unified launch plan

All existing launch entry points already converge on
`prepareBrowserStartPlan`. Phase 2 attaches a public LaunchPlan v2 at this
single boundary rather than duplicating launch compilation.

The public plan exposes only summaries:

- environment ID and revision;
- core capability manifest;
- configured/effective network mode;
- counts of persona/profile/temporary arguments;
- immediate/deferred target counts;
- restore-session policy;
- warnings and a digest.

It never returns raw proxy configuration, the managed user-data path, raw
arguments, or credentials.

The latest launch receipt records whether a plan is prepared, effective, or
failed and records CDP readiness. `BrowserProfileLaunchPlanState` reports
`configured`, `effective`, or `pending-next-launch` by comparing the
running receipt's environment revision with the current profile revision.

## Kernel capability manifest

A capability manifest is bound to the selected/default core and contains:

- core ID/name;
- browser version;
- SHA-256 of the actual executable when readable;
- host OS/architecture;
- the configuration channel;
- capability stage information.

Current code only promotes an existing setting to `emitted` when the compiler
actually produces a runtime argument. `acknowledged`, `observed`, and
`verified` remain false unless a later runtime-evidence path explicitly
supplies that proof. Matching a Chromium version or accepting a command-line
argument is not treated as verification.

## Compatibility

The current Profile type and legacy persistence stay in place during phase 2.
This makes rollback possible and keeps existing Wails/API/backup functionality
working while consumers migrate to the new DTOs.

No UI redesign, external sample ingestion, detector bypass logic, or browser
identity generation is added by this phase.
