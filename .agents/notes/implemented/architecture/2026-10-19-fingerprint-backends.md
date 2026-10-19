# Agent Note: Independent fingerprint backends

Status: implemented

## Problem

The frontend serializer and Go launch matrix assume fingerprint-chromium. Clearcote uses different switches and supports native overrides that the legacy matrix removes.

## Decision

Core metadata selects an explicit backend. Missing metadata on existing records keeps fingerprint-chromium; newly imported unknown executables use generic-chromium until detection or a manual choice establishes a contract. Native binary lookup, user data, proxies, extensions and CDP continue to use the existing launch path.

Each frontend adapter owns serialization, validation and capabilities. The Go launch adapter is authoritative for all callers, including API clients. Clearcote defaults to automatic Persona and emits hardware overrides only in manual mode. Backend-specific settings remain in the profile when switching cores.

The existing fingerprint argument array stores an AntBrowser-only `--ant-clearcote-config=<JSON>` record containing the mode, dormant manual values and optional managed Profile file reference. No BrowserProfile schema migration is needed. Every launch adapter consumes this record before spawning Chromium. Clearcote reconstructs missing native switches from the record after another frontend serializer has edited the profile; explicit raw switches take priority. The generic editor preserves the record while its launch adapter removes all fingerprint switches.

Imported fingerprint JSON is stored locally and encoded at launch as gzip level 9 plus base64, matching the official SDK. The payload is never persisted as profile configuration.

Capability probing uses an ephemeral HTTP page bound to `127.0.0.1`, a fresh user data directory and an OS-assigned CDP port. It waits for the secure page context before evaluating RAM and Touch. `about:blank` is empirically insecure in the tested build and does not expose `navigator.deviceMemory`; using it would misclassify a working Clearcote binary. The browser process tree, local page server, sockets and temporary profile are closed after the probe.

## Alternatives considered

Extending the legacy serializer is smaller but shares its no-effect matrix and silently removes valid Clearcote switches. Detecting by executable name/version is convenient but cannot distinguish Chromium-based builds; explicit metadata and an isolated capability probe provide the required evidence.

## Consequences

Frontend and backend have independent language-specific adapters and require contract tests. Existing launch behavior remains the compatibility baseline. A capability probe establishes Clearcote compatibility, not official provenance. Native integration results and remaining checks are recorded in the implementation report.

Windows native acceptance covers Clearcote 150.0.7871.114 and fingerprint-chromium 150.0.7871.186, including native overrides, proxy, extension, CDP, profile payload, concurrent isolation, restart and switching kernels on one user data directory. See [implementation and verification](../../../../CLEARCOTE-ADAPTATION.md).

No existing active decision notes were present when this note was created.
