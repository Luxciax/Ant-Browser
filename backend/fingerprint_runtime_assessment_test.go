package backend

import "testing"

func TestFingerprintAssessmentDoesNotClaimUnverifiedWorkerPassed(t *testing.T) {
	r := BrowserFingerprintRuntimeInfo{Language: "en-US", Languages: []string{"en-US"}, Timezone: "America/New_York", HardwareConcurrency: 8, Platform: "Win32", UserAgent: "test"}
	e := BrowserFingerprintExpectedInfo{Language: "en-US", Timezone: "America/New_York", HardwareConcurrency: "8", Platform: "windows"}
	if status, _ := assessFingerprintRuntime(r, e); status != "unverified" {
		t.Fatal(status)
	}
	r.Worker = &BrowserFingerprintWorkerInfo{Language: r.Language, Languages: r.Languages, Timezone: r.Timezone, HardwareConcurrency: r.HardwareConcurrency, Platform: r.Platform, UserAgent: r.UserAgent}
	if status, issues := assessFingerprintRuntime(r, e); status != "passed_basic" {
		t.Fatal(status, issues)
	}
	r.Worker.Timezone = "Asia/Shanghai"
	if status, _ := assessFingerprintRuntime(r, e); status != "mismatch" {
		t.Fatal(status)
	}
	r.Worker.Error = "CSP blocked"
	r.Timezone = "Asia/Shanghai"
	if status, _ := assessFingerprintRuntime(r, e); status != "mismatch" {
		t.Fatal(status)
	}
}
