# Security and runtime evidence

Source code is authoritative for authorization, secret storage and evidence semantics.

## Control-plane authentication

- LaunchServer authentication and scope mapping are implemented in `backend/internal/launchcode/auth.go`.
- Current scopes: `environment-read`, `page-read`, `page-action`, `script`, `environment-admin`, `backup-credential-export`, and `*`.
- API, MCP and CDP-facing control paths participate in authentication when enabled.
- Normal read responses must not expose raw proxy credentials, API keys or secret values.

## Secret storage

- Backup-channel credentials are stored through SecretRef.
- `backup.local.yaml` stores references, not new plaintext credentials.
- Windows uses the current SecretStore protection path; non-Windows requires the configured encryption key path.
- Encryption/protection failure must fail the save rather than silently falling back to plaintext.
- Profile packages do not make local backup credentials portable.

## Runtime evidence

Keep execution, observation and assertion separate:
- executed: a probe actually ran;
- observed: a usable value was produced;
- verified: an observed value matched a supported explicit expectation;
- mismatch: a supported expectation and observation disagree;
- unavailable/not-observed: evidence was not obtained.

Ordinary status reads do not become verification simply by returning configuration. Explicit runtime verification stores revision-bound evidence.

Cross-context probes must report applicability and execution independently. Unsupported or unexecuted contexts are not synthesized as passing.

## Network observation

- Host network observation may record DNS, TCP, TLS, ALPN, HTTP protocol/status and timing when actually observed.
- Loopback, private, link-local, multicast and other blocked targets remain protected by target validation/redirect checks.
- Host observations are not browser-route or server-seen observations.
- JA3/JA4, packet-level QUIC/HTTP3, controlled server-seen data and similar fields remain `not_observed` until dedicated external infrastructure supplies them.

## External compatibility

- A configured/emitted browser option is not proof of full graphics, font, device or network simulation.
- Detector scores and third-party anti-fraud outcomes are not release gates.
- Android/iOS/macOS or other cross-platform templates are not marked verified without a compatible runtime and corresponding observation evidence.
