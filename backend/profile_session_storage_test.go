package backend

import "testing"

func TestSessionStorageExceptionDoesNotExposeValues(t *testing.T) {
	params := map[string]any{"exceptionDetails": map[string]any{"url": "ant-session-restore.js", "exception": map[string]any{"description": "QuotaExceededError with synthetic-secret-value"}}}
	if got := sessionStorageExceptionReason(params); got != "quota_exceeded" {
		t.Fatal("not safely classified")
	}
	params["exceptionDetails"].(map[string]any)["url"] = "https://example.invalid/site.js"
	if sessionStorageExceptionReason(params) != "" {
		t.Fatal("unrelated website exception treated as migration failure")
	}
}
