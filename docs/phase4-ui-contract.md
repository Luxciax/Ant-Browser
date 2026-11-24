# Phase 4: UI integration contract

The presentation layer should consume `BrowserProfileEnvironmentView` instead
of reconstructing environment truth from unrelated profile fields.

EnvironmentView v2 contains:

- environment/persona/kernel/network/storage contract;
- frozen template binding and available templates;
- configured/effective/pending-next-launch state;
- latest launch readiness receipt when present;
- kernel capability evidence;
- latest explicit runtime evidence when present.

The frontend API module `api/environmentV2.ts` also exposes NetworkPolicy,
template application, and batch create/start/stop/query operations.

UI rules:

- configured values and effective running values must be shown separately;
- pending-next-launch must not be presented as already active;
- missing runtime evidence is not a failure and must not be shown as verified;
- template revision is immutable historical metadata until the user explicitly
  applies another template;
- per-item batch failures remain visible and retryable through operation IDs.

This phase intentionally does not prescribe colors, layout, typography or
component styling.
