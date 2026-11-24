# Phase 3: NetworkPolicy and lifecycle operations

## NetworkPolicy

Each profile has a persisted NetworkPolicy with conservative defaults:

- `failClosed=true`
- `dnsMode=inherit`
- `webRtcMode=restricted`
- `localeLinkMode=manual`

The policy is scoped to AntBrowser. It does not modify operating-system DNS,
firewall rules, TUN configuration, v2rayN, or other applications.

`failClosed` prevents a configured proxy launch from silently becoming a
direct browser route. `restricted` WebRTC adds the existing
`--disable-non-proxied-udp` launch policy. DNS mode is declarative in phase 3:
the current proxy core remains responsible for its own DNS behavior.

Locale linking is never silently applied. `recommend` permits UI/API clients
to show location-derived locale recommendations; `manual` keeps proxy
location and browser locale independent.

## Lifecycle operations

Phase 3 makes environment lifecycle actions explicit:

- clone as a new environment;
- clone while preserving the logical environment ID;
- refresh only the environment ID;
- preview a kernel switch;
- apply a kernel switch after validating the target executable.

These operations do not rewrite browser storage or Persona parameters unless
the requested operation already does so. Refreshing the environment ID clears
the in-memory launch receipt and runtime evidence because those records belong
to the previous lifecycle revision.

Package migration/restore remains the cross-machine migration path and now
carries environment IDs, NetworkPolicy, and frozen template ID/revision.
Kernel upgrade preview compares actual binary identity/version and warns when
runtime evidence should be repeated after restart.
