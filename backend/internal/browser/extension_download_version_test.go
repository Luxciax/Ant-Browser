package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestExtensionDownloadUsesConfiguredKernelVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":"151.0.7922.137"}`), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewManager(&config.Config{Browser: config.BrowserConfig{Cores: []config.BrowserCore{{CoreId: "core", CorePath: dir, IsDefault: true}}, DefaultCoreId: "core"}}, dir)
	version := m.extensionDownloadChromeVersion()
	u, err := url.Parse(BuildChromeExtensionDownloadURL("nngceckbapebfimnlniiiahkandclblb", version))
	if err != nil || u.Query().Get("prodversion") != "151.0.7922.137" {
		t.Fatalf("wrong download version: %v %s", err, u)
	}
	if strings.Contains(BuildChromeExtensionDownloadURL("nngceckbapebfimnlniiiahkandclblb"), "120.0.0.0") {
		t.Fatal("outdated fallback")
	}
}

func TestExtensionDownloadEmptyResponseReportsAvailabilityNotCorruptZip(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("User-Agent"), "Chrome/151.0.0.0") {
				t.Errorf("UA version mismatch: %s", r.Header.Get("User-Agent"))
			}
			w.WriteHeader(status)
		}))
		_, err := downloadChromeExtensionCRXOnce(context.Background(), s.Client(), s.URL+"?prodversion=151.0.0.0")
		s.Close()
		if err == nil || strings.Contains(err.Error(), "ZIP") || !strings.Contains(err.Error(), "151.0.0.0") {
			t.Fatalf("wrong empty response diagnostic: %v", err)
		}
	}
}

func TestDarwinCoreArchiveFiltersAvoidInvalidUTTypes(t *testing.T) {
	pattern := SupportedCoreArchiveDialogPatternForOS("darwin")
	if strings.Contains(pattern, "*.tar.") || strings.Contains(pattern, "*.*") || !strings.Contains(pattern, "*.gz") {
		t.Fatalf("unsafe Mac dialog filter: %s", pattern)
	}
	if SupportedCoreArchiveDialogPatternForOS("windows") != SupportedCoreArchivePattern() {
		t.Fatal("Windows filters changed")
	}
}
