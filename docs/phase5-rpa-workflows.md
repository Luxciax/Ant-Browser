# Phase 5: recoverable RPA workflows

Workflow definitions live under `data/automation/workflows`; run history lives
under `data/automation/workflow-runs`.

Supported control-flow step types are `action`, `set`, `if`, `loop`, and
`sleep`. Page actions reuse the existing long-lived PageSession rather than
introducing another browser-control path.

Retries are capped at five attempts. Side-effecting actions do not retry unless
`retry.allowSideEffects=true` is explicitly set.

Runs persist `nextStep` before and after each top-level step. On application
restart, `running` records become `interrupted`. Resume is refused by default
when the pending step can have side effects; the caller must explicitly force
that resume after inspecting current page/runtime state.

Breakpoints can be embedded in a step or supplied per run. A breakpoint returns
a persisted `paused` run that can be resumed.

Schedules support either `everyMinutes` or a local `dailyTime`. Overlap is
disabled unless explicitly enabled.
