# Setup

Ant Browser owns browser environments and runtime state. OpenClaw attaches through Ant Browser LaunchServer and its unified CDP endpoint.

## Preconditions

- Ant Browser and the OpenClaw gateway run on the same host.
- LaunchServer is reachable on loopback.
- If authentication is enabled, OpenClaw has the configured API key and header.
- The OpenClaw browser profile points to the LaunchServer CDP URL.

The default LaunchServer port is `19876`. Treat running application settings as authoritative if it was changed.

## Environment

```text
ANT_CHROME_BASE_URL=http://127.0.0.1:19876
ANT_CHROME_API_HEADER=X-Ant-Api-Key
ANT_CHROME_API_KEY=<only when auth is enabled>
```

Keep the key in the host secret/config mechanism rather than prompts or committed files.

## Check

1. Start Ant Browser.
2. Request `GET /api/health`.
3. Inspect `GET /api/runtime/active` if an existing CDP target matters.
4. Use `POST /api/runtime/session` or a launch endpoint to prepare the requested target.
5. Attach only after the response reports a ready CDP target.

For request examples, read `http.md`.

## Boundaries

- LaunchServer is intended for local control; do not assume a public deployment.
- Authentication scopes may reject a valid key that lacks the required operation scope.
- Single-target runtime selectors must resolve unambiguously.
- The complete route list is defined in `backend/internal/launchcode/server_http.go`, not this setup guide.
