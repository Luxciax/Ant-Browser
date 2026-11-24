# Phase 4: API/MCP capability scopes

The existing API key is now also an authorization principal. Supported scopes:

- `environment-read`
- `page-read`
- `page-action`
- `script`
- `environment-admin`
- `backup-credential-export`
- `*` for full access

Existing configurations with no explicit scopes normalize to `*` for
backward compatibility.

HTTP requests are authenticated first and then checked against the route's
required scope. Raw CDP proxy traffic requires `page-action`.

MCP exposes only tool groups allowed by the configured scopes. Tool annotations
remain descriptive; registration is filtered by the service authorization
state rather than trusting annotations.

The desktop control plane can rotate the API key with a new scope set or revoke
it. Rotation returns the newly generated key once; normal launch-server status
does not expose the key.
