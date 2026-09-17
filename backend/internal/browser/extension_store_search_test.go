package browser

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
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
