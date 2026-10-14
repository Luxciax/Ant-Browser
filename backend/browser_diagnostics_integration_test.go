package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/diagnostics"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMissingProxyFailsClosedButExplicitDirectStillWorks(t *testing.T) {
	a := &App{browserMgr: &browser.Manager{}, config: config.DefaultConfig()}
	p := &BrowserProfile{ProfileId: "missing-proxy-test", ProxyId: "deleted-proxy"}
	url, _, _, err := a.resolveBrowserStartProxy(browserStartInput{ProfileID: p.ProfileId}, p)
	if err == nil || url != "" || p.ProxyId != "deleted-proxy" {
		t.Fatalf("missing proxy silently accepted: %q %v", url, err)
	}
	url, _, _, err = a.resolveBrowserStartProxy(browserStartInput{ProfileID: p.ProfileId, ForceDirectProxy: true}, p)
	if err != nil || url != "direct://" {
		t.Fatalf("explicit direct broken: %q %v", url, err)
	}
}

// Opt-in native test. Uses a NEW headless process and temporary profile;
// never targets an existing browser or an external site.
func TestNativeBrowserDiagnosticsAndWorker(t *testing.T) {
	binary := os.Getenv("ANT_BROWSER_TEST_CHROME")
	if binary == "" {
		t.Skip("set ANT_BROWSER_TEST_CHROME to verify the actual configured kernel")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.Command(binary, "--headless=new", "--user-data-dir="+dir, "--remote-debugging-port=0", "--no-first-run", "--disable-background-networking", "--disable-component-update", "--disable-sync", "--no-proxy-server", "--timezone=America/New_York", "--fingerprint-hardware-concurrency=8", "about:blank")
	monitor, err := newBrowserProcessMonitor(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	monitor.Start()
	port := 0
	defer func() {
		cancel()
		if port > 0 {
			_ = cdpBrowserCall(port, "Browser.close", nil)
		}
		select {
		case <-monitor.Done():
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-monitor.Done()
		}
	}()
	port, err = waitBrowserDebugPortReadyContext(ctx, 0, dir, 8*time.Second, monitor)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan diagnostics.Event, 128)
	stopped := make(chan struct{})
	go func() {
		diagnostics.Observe(ctx, port, func(e diagnostics.Event) {
			select {
			case events <- e:
			default:
			}
		})
		close(stopped)
	}()
	waitEvent := func(name string) diagnostics.Event {
		t.Helper()
		timer := time.NewTimer(7 * time.Second)
		defer timer.Stop()
		for {
			select {
			case e := <-events:
				if e.Name == name {
					return e
				}
			case <-timer.C:
				t.Fatalf("native event %s missing", name)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	connected := waitEvent("diagnostics.connected")
	t.Logf("configured kernel=%s, runtime=%v", binary, connected.Fields["browser_version"])
	waitEvent("diagnostics.network_ready")
	runtimeInfo, err := evaluateBrowserFingerprintRuntime(port)
	if err != nil {
		t.Fatal(err)
	}
	status, issues := assessFingerprintRuntime(runtimeInfo, BrowserFingerprintExpectedInfo{Timezone: "America/New_York", HardwareConcurrency: "8"})
	if status != "passed_basic" {
		t.Fatalf("actual kernel basic spoof/Worker check: %s %v", status, issues)
	}
	verifyNativeDiagnosticEvents(t, port, waitEvent)
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("native observer did not stop")
	}
}

func verifyNativeDiagnosticEvents(t *testing.T, port int, waitEvent func(string) diagnostics.Event) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("local test refusal"))
	}))
	defer server.Close()
	if _, err := cdpCall(port, "Page.navigate", map[string]any{"url": server.URL + "/?token=PRIVATE_DIAGNOSTIC_TEST"}); err != nil {
		t.Fatal(err)
	}
	response := waitEvent("network.response")
	if response.Fields["http_status"] != 403 {
		t.Fatalf("HTTP refusal: %#v", response)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	failedURL := "http://" + listener.Addr().String() + "/private-test?token=PRIVATE_DIAGNOSTIC_TEST"
	listener.Close()
	if _, err = cdpCall(port, "Runtime.evaluate", map[string]any{"expression": "fetch(" + strconv.Quote(failedURL) + ").catch(()=>null)", "awaitPromise": true, "returnByValue": true}); err != nil {
		t.Fatal(err)
	}
	failure := waitEvent("network.failed")
	if failure.Fields["net_error"] != "net::ERR_CONNECTION_REFUSED" {
		t.Fatalf("missing Chromium error: %#v", failure)
	}
	// Page.crash applies only to this NEW disposable target. CDP may disconnect.
	_, _ = cdpCall(port, "Page.crash", nil)
	crash := waitEvent("renderer.crashed")
	encoded, _ := json.Marshal([]diagnostics.Event{response, failure, crash})
	if strings.Contains(string(encoded), "PRIVATE_DIAGNOSTIC_TEST") || strings.Contains(string(encoded), "private-test") {
		t.Fatal("network contents leaked into diagnostic events")
	}
	t.Logf("native HTTP403=%v; network failure=%v; renderer crash=%v", response.Fields["http_status"], failure.Fields["net_error"], crash.Fields["status"])
}

// Separate diagnostic acceptance from the independent fingerprint capability test.
func TestNativeKernelDiagnosticEvidence(t *testing.T) {
	binary := os.Getenv("ANT_BROWSER_TEST_CHROME")
	if binary == "" {
		t.Skip("set ANT_BROWSER_TEST_CHROME for native diagnostic acceptance")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.Command(binary, "--headless=new", "--user-data-dir="+dir, "--remote-debugging-port=0", "--no-first-run", "--disable-background-networking", "--disable-component-update", "--disable-sync", "--no-proxy-server", "about:blank")
	m, err := newBrowserProcessMonitor(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		m.closeCapture()
		t.Fatal(err)
	}
	m.Start()
	port := 0
	defer func() {
		cancel()
		if port > 0 {
			_ = cdpBrowserCall(port, "Browser.close", nil)
		}
		select {
		case <-m.Done():
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
			<-m.Done()
		}
		m.closeCapture()
	}()
	port, err = waitBrowserDebugPortReadyContext(ctx, 0, dir, 8*time.Second, m)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan diagnostics.Event, 256)
	done := make(chan struct{})
	go func() {
		diagnostics.Observe(ctx, port, func(e diagnostics.Event) {
			select {
			case events <- e:
			default:
			}
		})
		close(done)
	}()
	seen := map[string]bool{}
	wait := func(name string) diagnostics.Event {
		t.Helper()
		timer := time.NewTimer(7 * time.Second)
		defer timer.Stop()
		for {
			select {
			case e := <-events:
				seen[e.Name] = true
				if e.Name == name {
					return e
				}
			case <-timer.C:
				t.Fatalf("native event %s missing", name)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
	}
	connected := wait("diagnostics.connected")
	t.Logf("actual configured binary=%s version=%v", binary, connected.Fields["browser_version"])
	wait("diagnostics.network_ready")
	verifyNativeDiagnosticEvents(t, port, wait)
	if !seen["browser.gpu_info"] || !seen["browser.child_processes"] {
		t.Fatalf("missing GPU/child evidence: %v", seen)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("observer shutdown failed")
	}
}
