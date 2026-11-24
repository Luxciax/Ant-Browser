# Phase 5: network observations and compatibility template packs

The network observation endpoint is an explicit active probe. It currently
records host-source evidence only:

- DNS timing, answer count, address family and short address hashes;
- TCP connect network/remote endpoint/timing;
- TLS version, cipher suite, server name, negotiated ALPN and timing;
- response HTTP protocol/status and total duration.

The probe strips URL credentials, query strings and fragments from persisted
evidence. It sends no Ant cookies or account credentials and stores no response
body.

The following fields deliberately remain `not_observed` unless a future probe
collects direct evidence: JA3, JA4, QUIC, independent HTTP/3 proof, browser
proxy route and server-seen telemetry. Ant does not synthesize those values
from Chromium version, ALPN or another product's measurements.

Environment template packs now include Windows/macOS/Linux desktop
compatibility targets and Android/iOS mobile targets. These packs describe
target OS/device class/kernel family and required capability names. Applying a
pack does not fabricate a device identity and does not turn a desktop engine
into a mobile engine. Mobile packs remain `requires-compatible-kernel` until
the selected kernel/runtime actually exposes and verifies the required
capabilities.
