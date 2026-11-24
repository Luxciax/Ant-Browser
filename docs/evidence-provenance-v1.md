# Evidence provenance v1

This document defines the first-stage provenance contract used to organize
existing local research material without interpreting or transforming its
browser-fingerprint payload.

The contract exists to answer four questions before any experimental conclusion
is used elsewhere:

1. Which immutable run does a file belong to?
2. Which stage inside that run produced it?
3. Did the file change after the run manifest was created?
4. Are two manifests claiming the same run ID with different contents?

It is not a browser-fingerprint model, a profile generator, a detector-bypass
mechanism, or proof that an observation is correct.

## Files

Two read-only/local tools are available:

- `tools/evidence_inventory.py` inventories an input tree at file level.
- `tools/evidence_provenance.py` creates immutable run manifests and a run index.

Neither tool accesses the network. Neither writes into the selected input root.

## Run specification

A build specification is ordinary JSON:

```json
{
  "run_id": "ML-1504-20261120T101500Z",
  "sample_id": "ML-1504",
  "metadata": {
    "product": "research-source",
    "collector_version": "v3",
    "kernel_version": "unknown"
  },
  "stages": [
    {
      "stage_id": "A1",
      "ordinal": 0,
      "execution_status": "success",
      "started_at": "2026-11-20T10:15:00+00:00",
      "completed_at": "2026-11-20T10:16:00+00:00",
      "session_ref": "session-001",
      "artifacts": [
        "lifecycle_v2/ML-1504/A1/probe_results.json",
        "lifecycle_v2/ML-1504/A1/session_identity.json"
      ]
    }
  ],
  "reports": {
    "coverage": "lifecycle_v2/ML-1504/C2/coverage.json",
    "summary": "lifecycle_v2/ML-1504/C2/summary.json"
  }
}
```

`run_id` identifies one execution. `sample_id` may be reused across executions.
Do not use a sample directory name as a run ID when files in that directory came
from different executions.

`execution_status` is one of `success`, `partial`, `failed`, `skipped`, or
`unknown`. It describes execution only. It does not mean the measurement is
valid or that an assertion passed.

`session_ref` is an opaque, non-secret correlation label. Do not place account
IDs, cookies, proxy credentials, tokens, or other secrets in metadata.

## Build and verify

Example:

```powershell
python tools/evidence_provenance.py build   --root "D:\research\source"   --spec "D:\research\specs\run-001.json"   --output "D:\research\manifests\run-001.manifest.json"

python tools/evidence_provenance.py verify   --root "D:\research\source"   --manifest "D:\research\manifests\run-001.manifest.json"
```

The builder rejects duplicate stage IDs, non-increasing stage ordinals,
duplicate artifact assignment across stages, unsafe paths, links/reparse
points, impossible timestamp order, and files changed during hashing.

The output is created once and is not overwritten. Verification re-hashes the
current artifacts. A changed file does not silently become part of the old run.

## Run index

```powershell
python tools/evidence_provenance.py index   --manifests "D:\research\manifests\run-001.manifest.json"               "D:\research\manifests\run-002.manifest.json"   --output "D:\research\indexes\index-001.json"
```

A repeated `run_id` with different manifest digests is reported as a conflict.
Repeated `sample_id` values are allowed because one sample can be measured in
multiple independent runs.

## Observation and assertion status

Run provenance is intentionally separate from probe semantics. Downstream
reports should keep at least these three concepts separate:

- execution status: whether a probe actually ran;
- observation validity: whether it returned data satisfying the probe contract;
- assertion result: whether the valid observation matched a stated expectation.

`success`, `resolved`, or a complete manifest must never substitute for the
other two concepts.

## Final report convergence

Coverage, audit, diff, stability, and summary files may be included in the
`reports` map. Their hashes become part of the immutable run manifest so a
later rewrite cannot silently be presented as the original report.

The v1 tool does not parse report payloads or assert that their internal
statistics agree. Source-specific reconciliation requires a defined report
schema; until then, conflicting totals remain conflicts rather than being
auto-corrected.

## Migration of historical directories

For old datasets:

1. identify stage timestamps and existing session/process metadata;
2. create a run specification only when files can be assigned to one execution
   with reasonable evidence;
3. put ambiguous stages into separate candidate runs or leave them unassigned;
4. never invent missing stages, timestamps, or observations;
5. keep original files untouched and store manifests/indexes separately.

The output is provenance metadata, not permission to copy proprietary source
code, credentials, unique account identifiers, or commercial profile material
into AntBrowser.
