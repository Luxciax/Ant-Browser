package browser

import (
	"ant-chrome/backend/internal/config"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFingerprintDownloadAttribution(t *testing.T) {
	for _, item := range []struct {
		source string
		want   config.FingerprintBackend
	}{
		{"https://github.com/clearcotelabs/clearcote-browser/releases/download/v1/browser.zip", config.FingerprintBackendClearcote},
		{"https://github.com/adryfish/fingerprint-chromium/releases/download/v1/browser.zip", config.FingerprintBackendFingerprintChromium},
		{"https://github.com.evil.example/clearcotelabs/clearcote-browser/releases/download/v1/browser.zip", ""},
		{"https://example.com/clearcote.zip", ""},
	} {
		got, _ := FingerprintBackendForDownload(item.source)
		if got != item.want {
			t.Fatalf("incorrect attribution for %s: %s", item.source, got)
		}
	}
}

func TestCoreFingerprintManifest(t *testing.T) {
	root := t.TempDir()
	manifest := CoreFingerprintManifest{Provider: "clearcote-compatible", FingerprintBackend: config.FingerprintBackendClearcote, ChromiumVersion: "150.0.0.0"}
	if err := WriteCoreFingerprintManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadCoreFingerprintManifest(root)
	if !ok || got.FingerprintBackend != manifest.FingerprintBackend || got.Provider != manifest.Provider || got.DetectedAt == "" {
		t.Fatalf("invalid manifest: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "ant-core.json"), []byte(`{"schemaVersion":999,"fingerprintBackend":"clearcote"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadCoreFingerprintManifest(root); ok {
		t.Fatal("unknown schemas must not establish a backend")
	}
}

func TestCoreProbeFailureCleansTemporaryProfile(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := ProbeClearcoteCompatibility(ctx, filepath.Join(root, "missing-chrome"), root); err == nil {
		t.Fatal("missing binary must fail probe")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe leaked temporary profile: %v %v", entries, err)
	}
}
