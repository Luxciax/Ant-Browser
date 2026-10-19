package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Opt-in acceptance test: actual native kernels, only task-owned profiles and a local proxy.
func TestNativeFingerprintBackendAcceptance(t *testing.T) {
	clearBinary, legacyBinary := os.Getenv("ANT_TEST_CLEARCOTE"), os.Getenv("ANT_TEST_FINGERPRINT_CHROMIUM")
	if clearBinary == "" || legacyBinary == "" {
		t.Skip("set ANT_TEST_CLEARCOTE and ANT_TEST_FINGERPRINT_CHROMIUM for native acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	probeRoot := t.TempDir()
	if err := browser.ProbeClearcoteCompatibility(ctx, clearBinary, probeRoot); err != nil {
		t.Fatalf("Clearcote capability probe: %v", err)
	}
	if entries, err := os.ReadDir(probeRoot); err != nil || len(entries) != 0 {
		t.Fatalf("probe profile leak: %v %v", entries, err)
	}

	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>Ant local proxy</title></head><body>kernel test</body></html>"))
	}))
	defer proxy.Close()
	extension := t.TempDir()
	for name, contents := range map[string]string{
		"manifest.json": `{"manifest_version":3,"name":"Ant kernel acceptance","version":"1.0","content_scripts":[{"matches":["http://localhost/*"],"js":["content.js"],"run_at":"document_end"}]}`,
		"content.js":    `document.documentElement.dataset.antExtension='loaded';`,
	} {
		if err := os.WriteFile(filepath.Join(extension, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	launch := func(binary string, backend config.FingerprintBackend, id, userDir string, raw []string) (int, []string, func()) {
		t.Helper()
		cfg := config.DefaultConfig()
		cfg.Browser.Cores = []config.BrowserCore{{CoreId: "kernel", CoreName: "Test kernel", CorePath: binary, IsDefault: true, FingerprintBackend: backend}}
		app := NewApp(t.TempDir())
		app.config = cfg
		app.browserMgr = browser.NewManager(cfg, app.appRoot)
		report := app.buildBrowserFingerprintCapabilityReport(id, "kernel", raw)
		native := report.LaunchArgs
		var err error
		if backend == config.FingerprintBackendClearcote {
			native, err = prepareClearcoteProfileArgs(raw, native)
			if err != nil {
				t.Fatal(err)
			}
		}
		assignedPort, err := nextAvailablePort()
		if err != nil {
			t.Fatal(err)
		}
		args := buildBrowserLaunchArgs(userDir, assignedPort, proxy.URL, []string{extension}, native, nil, []string{"--headless=new", "--no-first-run", "--disable-background-networking", "--disable-component-update", "--disable-sync", "--proxy-bypass-list=<-loopback>", "--disable-extensions-except=" + extension}, []string{"about:blank"}, false)
		for _, arg := range args {
			if strings.HasPrefix(arg, "--ant-") {
				t.Fatalf("application metadata leaked to browser: %s", arg)
			}
		}
		cmd := exec.Command(binary, args...)
		monitor, err := newBrowserProcessMonitor(cmd)
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		monitor.Start()
		port, err := waitBrowserDebugPortReadyContext(ctx, assignedPort, userDir, 8*time.Second, monitor)
		var closeOnce sync.Once
		closeBrowser := func() {
			closeOnce.Do(func() {
				if port > 0 {
					_ = cdpBrowserCall(port, "Browser.close", nil)
				}
				select {
				case <-monitor.Done():
				case <-time.After(2 * time.Second):
					_ = cmd.Process.Kill()
					<-monitor.Done()
				}
			})
		}
		if err != nil {
			closeBrowser()
			t.Fatal(err)
		}
		t.Cleanup(closeBrowser)
		return port, native, closeBrowser
	}
	evaluate := func(port int, expression string) any {
		t.Helper()
		result, err := cdpCall(port, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true})
		if err != nil {
			t.Fatal(err)
		}
		value, ok := result["result"].(map[string]any)
		if !ok {
			t.Fatalf("no CDP result: %v", result)
		}
		return value["value"]
	}
	navigate := func(port int) {
		t.Helper()
		if _, err := cdpCall(port, "Page.navigate", map[string]any{"url": "http://localhost:65530/acceptance"}); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			if evaluate(port, "document.title==='Ant local proxy' && document.documentElement.dataset.antExtension==='loaded'") == true {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("proxy navigation or unpacked extension did not work: title=%v extension=%v", evaluate(port, "document.title"), evaluate(port, "document.documentElement.dataset.antExtension"))
	}
	raw := []string{"--fingerprint-platform=windows", "--fingerprint-brand=Chrome", "--fingerprint-hardware-concurrency=8", "--fingerprint-screen-width=1366", "--fingerprint-screen-height=768", "--fingerprint-avail-width=1366", "--fingerprint-avail-height=728", "--fingerprint-device-pixel-ratio=1.25", "--fingerprint-max-touch-points=7", "--timezone=Asia/Tokyo", "--accept-lang=en-US,en", "--fingerprint-gpu-vendor=Ant Test Vendor", "--fingerprint-gpu-renderer=Ant Test Renderer", clearcoteConfigArg + `{"mode":"manual","deviceMemory":"4"}`}
	userDir := t.TempDir()
	port, args, closeFirst := launch(clearBinary, config.FingerprintBackendClearcote, "stable-profile", userDir, raw)
	navigate(port)
	runtimeInfo, err := evaluateBrowserFingerprintRuntime(port)
	if err != nil {
		t.Fatal(err)
	}
	if runtimeInfo.HardwareConcurrency != 8 || runtimeInfo.DeviceMemory != 4 || runtimeInfo.ScreenWidth != 1366 || runtimeInfo.ScreenHeight != 768 || runtimeInfo.AvailWidth != 1366 || runtimeInfo.AvailHeight != 728 || runtimeInfo.DevicePixelRatio != 1.25 || runtimeInfo.MaxTouchPoints != 7 || runtimeInfo.Timezone != "Asia/Tokyo" || runtimeInfo.WebGLVendor != "Ant Test Vendor" || runtimeInfo.WebGLRenderer != "Ant Test Renderer" {
		t.Fatalf("Clearcote native overrides: %+v", runtimeInfo)
	}
	expected := clearcoteKernelAdapter{}.expected(args)
	if status, issues := assessFingerprintRuntime(runtimeInfo, expected); status == "mismatch" {
		t.Fatalf("adapter expectations disagree with native runtime: %v", issues)
	}
	evaluate(port, "localStorage.setItem('ant-isolation','saved');document.cookie='ant_cookie=saved;max-age=3600';true")
	// A second kernel remains open concurrently and has an independent UserDataDir.
	legacyPort, _, _ := launch(legacyBinary, config.FingerprintBackendFingerprintChromium, "legacy-profile", t.TempDir(), []string{"--fingerprint=54321", "--fingerprint-platform=windows", "--fingerprint-hardware-concurrency=6", "--timezone=Asia/Tokyo"})
	navigate(legacyPort)
	if value := evaluate(legacyPort, "localStorage.getItem('ant-isolation')"); value != nil {
		t.Fatalf("profile storage leaked across kernels: %v", value)
	}
	if value := evaluate(legacyPort, "navigator.hardwareConcurrency"); value != float64(6) {
		t.Fatalf("legacy CPU override regressed: %v", value)
	}
	closeFirst()
	rollbackPort, _, closeRollback := launch(legacyBinary, config.FingerprintBackendFingerprintChromium, "stable-profile", userDir, []string{"--fingerprint=9876"})
	navigate(rollbackPort)
	if evaluate(rollbackPort, "localStorage.getItem('ant-isolation')") != "saved" {
		t.Fatal("switching to legacy kernel changed user data")
	}
	closeRollback()
	restoredPort, restoredArgs, _ := launch(clearBinary, config.FingerprintBackendClearcote, "stable-profile", userDir, raw)
	navigate(restoredPort)
	if browserArgValue(args, "--fingerprint") != browserArgValue(restoredArgs, "--fingerprint") {
		t.Fatal("seed changed on restart")
	}
	if evaluate(restoredPort, "localStorage.getItem('ant-isolation')") != "saved" || !strings.Contains(evaluate(restoredPort, "document.cookie").(string), "ant_cookie=saved") {
		t.Fatal("profile session did not survive restart")
	}
	// The SDK payload is accepted by the real kernel, with explicit manual flags taking priority.
	profileFile := filepath.Join(t.TempDir(), "device.json")
	if err = os.WriteFile(profileFile, []byte(`{"hardware_concurrency":2,"device_memory":2,"screen":{"width":1920,"height":1080}}`), 0600); err != nil {
		t.Fatal(err)
	}
	profileMetadata, _ := json.Marshal(map[string]any{"mode": "profile", "fingerprintProfile": map[string]string{"source": "file", "filePath": profileFile}})
	importedPort, _, _ := launch(clearBinary, config.FingerprintBackendClearcote, "profile-only", t.TempDir(), []string{clearcoteConfigArg + string(profileMetadata)})
	if evaluate(importedPort, "navigator.hardwareConcurrency") != float64(2) || evaluate(importedPort, "screen.width") != float64(1920) {
		t.Fatal("imported native Profile did not take effect")
	}
	metadata, _ := json.Marshal(map[string]any{"mode": "manual", "fingerprintProfile": map[string]string{"source": "file", "filePath": profileFile}})
	profilePort, _, _ := launch(clearBinary, config.FingerprintBackendClearcote, "imported-profile", t.TempDir(), []string{"--fingerprint-hardware-concurrency=8", clearcoteConfigArg + string(metadata)})
	if evaluate(profilePort, "navigator.hardwareConcurrency") != float64(8) {
		t.Fatal("explicit flag must override imported Profile")
	}
	if requests.Load() == 0 {
		t.Fatal("local proxy was bypassed")
	}
	t.Log("native Clearcote + fingerprint-chromium: CDP, proxy, extension, CPU/RAM/screen/DPR/touch/GPU/timezone, profile payload, stable seed, isolation and restart passed")
}
