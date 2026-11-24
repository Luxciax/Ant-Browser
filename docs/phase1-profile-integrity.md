# Phase 1: profile data integrity and local API hardening

Baseline: `ff0115b22ef10ab60b38d8796f17c2efa0a76a77`.

This change is deliberately limited to the existing Go/Wails application.
It does not add browser identity generation, new fingerprint algorithms,
network bypass mechanisms, external sample extraction, or UI redesign.

## Sparse profile updates

- `ant_instance_update` and the new HTTP `PATCH /api/profiles/{profileId}`
  share the same sparse-update service.
- An omitted or null property leaves the corresponding value unchanged.
  An explicit empty array clears tags or keywords; a supplied numeric zero is
  processed by the existing memory-limit normalization policy.
- Full HTTP PUT remains full replacement and now requires all writable fields
  explicitly, with non-null values. Use PATCH for partial changes. Create
  remains unchanged. A running profile is not changed by the sparse update
  operation. An explicitly empty proxyId selects the existing direct policy;
  omit proxyId to retain the existing proxy binding.
- The manager applies patches while holding its lock, only changes requested
  fields, and writes one record using SQLite. YAML mode retains rollback on
  save failure. Session-restore mode can also be updated independently.
- When the optional launch code is requested, invalid codes are rejected before
  profile writes; an unsuccessful profile patch attempts to restore the
  previous launch-code mapping without performing a full profile replacement.

Example request:

```json
{
  "profile": {
    "profileName": "Research workspace",
    "tags": []
  }
}
```

## External profile responses

The profile object in MCP instance detail and HTTP profile management responses
is now an explicit allowlist. It omits raw proxy configuration, subscription
URLs, local storage paths, raw launch arguments, and diagnostic strings.
Internal application objects and backup formats are unchanged. Runtime control
routes retain their existing dedicated runtime information.

Consumers must not depend on private configuration fields being returned by
ordinary profile queries. The HTTP full-replacement route accepts explicit complete inputs only; the
response is not a replacement request. Supply empty arrays to clear lists,
not null. Ordinary clients should use PATCH.

## Managed launch parameters and authentication

Every external launch argument list is filtered at final browser argument
assembly, without filtering the trusted arguments constructed by the
application itself. The existing input preparation logs managed overrides.
Authentication, when requested, fails closed if its key is not configured.
Requests supplying multiple authentication header values are rejected.
Configured authentication covers all routes, including the catch-all CDP
proxy. The production listener validates loopback Host and port, rejects
cross-origin/cross-site browser requests, and does not forward the gateway
credential to CDP. JSON control responses are not cacheable.

## Archive preflight

Before preview or import, the authoritative instance catalog is checked for
empty or invalid IDs, duplicate normalized IDs, and a supplied manifest
count that disagrees with the actual catalog. The original configuration and
archive format are retained; no seed generation or browser identity algorithm
is changed by this work.

## Local evidence inventory

`tools/evidence_inventory.py` is a standard-library Python utility. Run it
only on a user-selected local input directory; the result is a separate new
JSON file outside that directory. For example (PowerShell):

```powershell
python tools/evidence_inventory.py "D:\research\input" --output "D:\research\reports\inventory-001.json"
python tools/evidence_inventory.py "D:\research\input" --output "D:\research\reports\inventory-002.json" --previous "D:\research\reports\inventory-001.json"
```

It records relative paths, sizes, timestamps, and SHA-256, detects files
changed during reading, skips links and special files, and verifies a
previous report before comparing paths. Exit 2 means some entries were not
verified; exit 1 means the inventory could not be produced. The reports may
contain private path names and should remain local. File-level integrity
does not establish that experimental stages belong to a single execution,
nor does it establish the correctness of individual observations. Historical
research pipeline reconciliation is not implemented here. No source files
are modified, and the tool does not access the network or parse the contents
into browser configuration.

## Scope and validation status

These commits implement the separable profile integrity, archive preflight,
parameter protection, local evidence inventory, and ordinary profile-response
hardening work. The external research pipeline and raw data are not in this
repository and are not uploaded by these changes. No test suite, build, workflow dispatch or external
sample collection was performed as part of the commit.
