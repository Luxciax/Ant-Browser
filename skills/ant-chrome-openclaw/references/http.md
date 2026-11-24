# HTTP examples

These are invocation examples, not a second API specification. Route registration and authorization remain authoritative in `backend/internal/launchcode/server_http.go` and `auth.go`.

Defaults:
```text
ANT_CHROME_BASE_URL=http://127.0.0.1:19876
ANT_CHROME_API_HEADER=X-Ant-Api-Key
ANT_CHROME_API_KEY=<when enabled>
```

## Windows helper

```powershell
pwsh -File skills/ant-chrome-openclaw/scripts/invoke_ant_chrome_api.ps1 -Method GET -Path /api/health
```

For JSON requests:
```powershell
$body = '{"selector":{"code":"BUYER_001"}}'
pwsh -File skills/ant-chrome-openclaw/scripts/invoke_ant_chrome_api.ps1 -Method POST -Path /api/runtime/session -JsonBody $body
```

## curl

```bash
BASE_URL="${ANT_CHROME_BASE_URL:-http://127.0.0.1:19876}"
API_HEADER="${ANT_CHROME_API_HEADER:-X-Ant-Api-Key}"
curl_ant() {
  if [ -n "${ANT_CHROME_API_KEY:-}" ]; then
    curl -sS -H "${API_HEADER}: ${ANT_CHROME_API_KEY}" "$@"
  else
    curl -sS "$@"
  fi
}
```

Health:
```bash
curl_ant "$BASE_URL/api/health"
```

Prepare a runtime session:
```bash
curl_ant -H 'Content-Type: application/json' -X POST "$BASE_URL/api/runtime/session" --data-raw '{"selector":{"code":"BUYER_001"}}'
```

Runtime status:
```bash
curl_ant -H 'Content-Type: application/json' -X POST "$BASE_URL/api/runtime/status" --data-raw '{"selector":{"code":"BUYER_001"}}'
```

Stop the Ant Browser instance:
```bash
curl_ant -H 'Content-Type: application/json' -X POST "$BASE_URL/api/runtime/stop" --data-raw '{"selector":{"code":"BUYER_001"}}'
```

Run a saved script with its stored target/defaults:
```bash
curl_ant -H 'Content-Type: application/json' -X POST "$BASE_URL/api/automation/scripts/run" --data-raw '{"scriptId":"news-query-txt"}'
```

List workflows:
```bash
curl_ant "$BASE_URL/api/automation/workflows"
```

Before retrying a failure, inspect the corresponding launch/script/workflow run records rather than blindly repeating a side-effecting operation.
