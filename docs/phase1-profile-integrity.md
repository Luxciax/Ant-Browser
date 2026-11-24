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
- Full HTTP PUT continues to represent full replacement. Create remains
  unchanged. A running profile is not changed by the sparse update operation.
- The manager applies patches while holding its lock, only changes requested
  fields, and restores the prior in-memory state if persistence fails.
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
ordinary profile queries. The HTTP full-replacement route remains compatible
for callers that already possess all required input fields.

## Managed launch parameters and authentication

Every external launch argument list is filtered at final browser argument
assembly, without filtering the trusted arguments constructed by the
application itself. The existing input preparation logs managed overrides.
Authentication, when requested, fails closed if its key is not configured.
Requests supplying multiple authentication header values are rejected.

## Scope and validation status

This commit implements the separable profile integrity, parameter protection,
and ordinary profile-response hardening work. The external commercial-browser
research pipeline and its raw data are not in this repository and are not
uploaded by this change. No test suite, build, workflow dispatch or external
sample collection was performed as part of the commit.
