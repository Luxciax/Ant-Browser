package browser

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type extensionRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn extensionRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestParseChromeWebStoreSearchResults(t *testing.T) {
	markup := `<main>
<div data-item-id="dhdgffkkebhmkfjojejmpbldmpobfkfo"><a href="./detail/tampermonkey/dhdgffkkebhmkfjojejmpbldmpobfkfo"></a><img src="https://example.test/tamper.png"><h2>Tampermonkey</h2></div>
<div data-item-id="ndcooeababalnlpkfedmmbbbgkljhpjf"><img src="https://example.test/scriptcat.png"><h2>ScriptCat</h2></div>
</main>`
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	results := parseChromeWebStoreSearchResults(doc, 12)
	if len(results) != 2 {
		t.Fatalf("results = %#v, want 2", results)
	}
	if results[0].ExtensionID != "dhdgffkkebhmkfjojejmpbldmpobfkfo" || results[0].Name != "Tampermonkey" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[0].IconURL != "https://example.test/tamper.png" {
		t.Fatalf("icon = %q", results[0].IconURL)
	}
}

func TestLookupExtensionMarksDownloadFailureNotInstallable(t *testing.T) {
	client := &http.Client{Transport: extensionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	manager := &Manager{}
	result, err := manager.LookupExtensionWithHTTPClient("dhdgffkkebhmkfjojejmpbldmpobfkfo", client)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installable {
		t.Fatalf("result = %#v, failed download must not be installable", result)
	}
	if !strings.Contains(result.Message, "HTTP 503") {
		t.Fatalf("message = %q, want HTTP failure detail", result.Message)
	}
}

func TestNewExtensionDefaultsToInstallForProfiles(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manifest.Write([]byte(`{"manifest_version":3,"name":"Fixture","version":"1.0.0"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{AppRoot: t.TempDir()}
	extension, err := manager.InstallExtensionPackageBytes("abcdefghijklmnopabcdefghijklmnop", "fixture.zip", buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !extension.Enabled || !extension.DefaultInstall {
		t.Fatalf("extension = %#v, new installs should be enabled and default-installed", extension)
	}
}

func TestLocalExtensionDirectoryDefaultsToInstallForProfiles(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "local-extension")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "manifest.json"), []byte(`{"manifest_version":3,"name":"Local Fixture","version":"1.0.0","key":"test-key"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}

	manager := &Manager{AppRoot: t.TempDir()}
	extension, err := manager.InstallExtensionDirectory(sourceDir)
	if err != nil {
		t.Fatalf("InstallExtensionDirectory: %v", err)
	}
	if !extension.Enabled || !extension.DefaultInstall {
		t.Fatalf("extension = %#v, local directory installs should be enabled and default-installed", extension)
	}
}

func TestLocalExtensionDirectoryKeepsStableIDAcrossManifestUpdates(t *testing.T) {
	appRoot := t.TempDir()
	sourceDir := filepath.Join(t.TempDir(), "stable-local-extension")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeManifest := func(version string) {
		t.Helper()
		data := []byte(`{"manifest_version":3,"name":"Stable Local Fixture","version":"` + version + `"}`)
		if err := os.WriteFile(filepath.Join(sourceDir, "manifest.json"), data, 0o644); err != nil {
			t.Fatalf("WriteFile manifest %s: %v", version, err)
		}
	}
	writeManifest("1.0.0")

	manager := &Manager{AppRoot: appRoot}
	manager.ExtensionDAO = newTestExtensionDAO(t, appRoot)
	first, err := manager.InstallExtensionDirectory(sourceDir)
	if err != nil {
		t.Fatalf("first InstallExtensionDirectory: %v", err)
	}
	if NormalizeExtensionID(first.ExtensionID) == "" {
		t.Fatalf("first extension id = %q, want valid Chrome extension id", first.ExtensionID)
	}
	keyPath := manager.localExtensionKeyPath(first.ExtensionID)
	firstKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read generated local extension key: %v", err)
	}
	managedManifest, err := os.ReadFile(filepath.Join(first.InstallDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read managed manifest: %v", err)
	}
	if !strings.Contains(string(managedManifest), `"key"`) {
		t.Fatalf("managed manifest does not contain stable public key: %s", managedManifest)
	}

	writeManifest("2.0.0")
	second, err := manager.InstallExtensionDirectory(sourceDir)
	if err != nil {
		t.Fatalf("second InstallExtensionDirectory: %v", err)
	}
	if second.ExtensionID != first.ExtensionID {
		t.Fatalf("extension id changed across manifest update: first=%s second=%s", first.ExtensionID, second.ExtensionID)
	}
	if second.Version != "2.0.0" {
		t.Fatalf("updated extension version = %q, want 2.0.0", second.Version)
	}
	secondKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read reused local extension key: %v", err)
	}
	if !bytes.Equal(firstKey, secondKey) {
		t.Fatal("local extension private key changed across manifest update")
	}
}
