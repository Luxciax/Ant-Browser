package backend

import (
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIncidentPersistenceRoundTripPreservesTimeAndHistoricalTimeline(t *testing.T) {
	root := t.TempDir()
	app := NewApp(root)
	id := "incident-20261014T100000.000000000-1"
	started := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	app.incidents = map[string]*diagnosticIncident{id: {ID: id, ProfileID: "historical-profile", RunID: "historical-run", Event: "renderer.crashed", Started: started, Updated: started, Status: "complete", Coverage: map[string]string{"cause": "unknown"}, Facts: []map[string]interface{}{{"authorization": "fake-private-token"}}, Timeline: []logger.MemoryLogEntry{{Time: started.Format(time.RFC3339Nano), Level: "ERROR", Message: "historical-fault", Fields: map[string]interface{}{"event_id": "historical-only"}}}}}
	app.incidentOrder = []string{id}
	app.refreshAndPersistIncident(id, true)
	raw, err := os.ReadFile(filepath.Join(app.incidentDir(), id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "fake-private-token") {
		t.Fatal("secret persisted")
	}
	var saved diagnosticIncident
	if err = json.Unmarshal(raw, &saved); err != nil || !saved.Started.Equal(started) {
		t.Fatal("invalid timestamp", err, string(raw))
	}
	restored := NewApp(root)
	list := restored.GetBrowserDiagnosticIncidents()
	if len(list) != 1 {
		t.Fatal(list)
	}
	report, err := restored.incidentReport(id)
	if err != nil {
		t.Fatal(err)
	}
	rows := report["timeline"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["message"] != "historical-fault" {
		t.Fatal("historical timeline erased", report)
	}
	if len(restored.GetBrowserDiagnosticRunLogs("historical-profile", "historical-run")) != 1 {
		t.Fatal("run retrieval missed saved fault")
	}
}

func TestIncidentDiskFailureDoesNotPreventMemoryExport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "data"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(root)
	id := "incident-disk-failure"
	app.incidents = map[string]*diagnosticIncident{id: {ID: id, ProfileID: "disk-test", RunID: "disk-run", Started: time.Now(), Coverage: map[string]string{}, Timeline: []logger.MemoryLogEntry{}}}
	app.incidentOrder = []string{id}
	app.incidentLoaded = true
	report, err := app.incidentReport(id)
	if err != nil {
		t.Fatal(err)
	}
	if report["coverage"].(map[string]interface{})["persistence"] == nil {
		t.Fatal("disk failure not explicit", report)
	}
}

func TestIncidentLoadRejectsUntrustedIdentityAndRotationKeepsOtherFiles(t *testing.T) {
	root := t.TempDir()
	a := NewApp(root)
	dir := a.incidentDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	alien := filepath.Join(dir, "incident-user-asset.json")
	if err := os.WriteFile(alien, []byte("{\"producer\":\"other-tool\"}"), 0600); err != nil {
		t.Fatal(err)
	}
	malicious := map[string]interface{}{"producer": incidentProducer, "incident_id": "incident-../../outside", "occurred_at": time.Now().Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(malicious)
	os.WriteFile(filepath.Join(dir, "incident-malicious.json"), raw, 0600)
	if len(a.GetBrowserDiagnosticIncidents()) != 0 {
		t.Fatal("accepted mismatched or escaping identity")
	}
	a.trimIncidentFiles(dir)
	if _, err := os.Stat(alien); err != nil {
		t.Fatal("unrelated user file removed", err)
	}
}
