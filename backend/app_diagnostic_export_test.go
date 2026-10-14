package backend

import (
	"ant-chrome/backend/internal/logger"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticReportPreservesJSONSchemaAndRedactsCredentials(t *testing.T) {
	logger.Init(context.Background(), "info")
	logger.New("BrowserDiagnostics").Error("network.failed", logger.F("profile_id", "profile-test"), logger.F("authorization", "private-test-token"), logger.F("url", "https://user:secret@example.com/?token=private-test-token"))
	report, err := (&App{}).diagnosticReport()
	if err != nil {
		t.Fatal(err)
	}
	logs, ok := report["logs"].([]any)
	if !ok || len(logs) == 0 {
		t.Fatalf("missing logs: %#v", report)
	}
	last := logs[len(logs)-1].(map[string]any)
	if last["time"] == nil || last["level"] != "ERROR" || last["fields"] == nil {
		t.Fatalf("invalid JSON log schema: %#v", last)
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "private-test-token") || strings.Contains(string(encoded), "user:secret") {
		t.Fatal("credential leaked")
	}
}
