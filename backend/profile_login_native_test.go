package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func startLoginTestKernel(t *testing.T, dir string, gated bool) int {
	return startLoginTestKernelMode(t, dir, gated, true)
}

func startLoginTestKernelMode(t *testing.T, dir string, gated, headless bool) int {
	t.Helper()
	kernel := os.Getenv("ANT_IDENTITY_TEST_CHROME")
	if kernel == "" {
		t.Skip("set ANT_IDENTITY_TEST_CHROME to the explicitly configured kernel")
	}
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--no-first-run", "--no-default-browser-check", "--disable-background-networking", fmt.Sprintf("--user-data-dir=%s", dir), fmt.Sprintf("--remote-debugging-port=%d", port)}
	if headless {
		args = append(args, "--headless=new")
	}
	if gated {
		args = append(args, "--no-startup-window")
	} else {
		args = append(args, "about:blank")
	}
	cmd := exec.Command(kernel, args...)
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cdpBrowserCall(port, "Browser.close", nil)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	deadline := time.Now().Add(12 * time.Second)
	for {
		if _, err := cdpGetEndpointBody(port, "/json/version"); err == nil {
			return port
		}
		select {
		case <-done:
			t.Fatal("kernel exited before CDP ready")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("kernel CDP readiness timeout")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestLoginStartupGateConfiguredKernel(t *testing.T) {
	port := startLoginTestKernelMode(t, t.TempDir(), true, false)
	data, err := cdpGetEndpointBody(port, "/json/list")
	if err != nil {
		t.Fatal(err)
	}
	var targets []cdpTarget
	if json.Unmarshal(data, &targets) != nil || len(targets) != 0 {
		t.Fatal("native startup gate opened a page before cookie restoration")
	}
}

func TestLoginMigrationUIConfiguredKernel(t *testing.T) {
	uiURL := os.Getenv("ANT_LOGIN_UI_URL")
	if uiURL == "" {
		t.Skip("requires a local UI fixture URL; synthetic bindings only")
	}
	port := startLoginTestKernel(t, t.TempDir(), false)
	eval := func(expression string) string {
		r, err := cdpCall(port, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true})
		if err != nil || r["exceptionDetails"] != nil {
			t.Fatal("fixture JavaScript failed")
		}
		result, _ := r["result"].(map[string]any)
		return fmt.Sprint(result["value"])
	}
	wait := func(expression string) {
		deadline := time.Now().Add(8 * time.Second)
		for eval(expression) != "true" {
			if time.Now().After(deadline) {
				t.Fatal("UI condition failed: " + expression)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	for _, mode := range []string{"export", "restore"} {
		url := uiURL
		if mode == "restore" {
			url += "?restore"
		}
		if _, err := cdpCall(port, "Page.navigate", map[string]any{"url": url}); err != nil {
			t.Fatal(err)
		}
		wait(`!!document.querySelector('input')`)
		if mode == "export" {
			eval(`document.querySelector('input[type=checkbox]').click();true`)
			wait(`document.querySelectorAll('input[type=password]').length===2`)
			eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='导出').click();true`)
			wait(`document.body.innerText.includes('迁移密码至少需要')`)
		} else {
			eval(`var input=document.querySelector('input[type=password]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,'synthetic-wrong-password');input.dispatchEvent(new Event('input',{bubbles:true}));true`)
			eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='解密并恢复备份').click();true`)
			wait(`document.querySelector('[role=alert]')?.textContent.includes('迁移密码不正确')===true`)
		}
		for _, width := range []int{1440, 800} {
			if _, err := cdpCall(port, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": false}); err != nil {
				t.Fatal(err)
			}
			screenshot, err := cdpCall(port, "Page.captureScreenshot", map[string]any{"format": "png"})
			if err != nil {
				t.Fatal(err)
			}
			png, err := base64.StdEncoding.DecodeString(screenshot["data"].(string))
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join("..", ".impeccable", "review")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("login-%s-%d.png", mode, width)), png, 0600); err != nil {
				t.Fatal(err)
			}
			// The incumbent modal ::after sheen deliberately extends beyond its
			// clipped box (index.css:346). Measure real content and viewport bounds,
			// not that decorative pseudo-element's scroll overflow.
			if eval(`Array.from(document.querySelectorAll('.modal-surface > *')).every(e=>e.scrollWidth<=e.clientWidth)&&document.querySelector('.modal-surface').getBoundingClientRect().left>=0&&document.querySelector('.modal-surface').getBoundingClientRect().right<=innerWidth`) != "true" {
				t.Fatal("modal content or viewport overflow")
			}
		}
		if mode == "restore" {
			eval(`var input=document.querySelector('input[type=password]');Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(input,'synthetic-correct-password');input.dispatchEvent(new Event('input',{bubbles:true}));true`)
			eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='解密并恢复备份').click();true`)
			wait(`document.querySelector('#done')?.textContent==='true'`)
			wait(`document.querySelector('input[type=password]')?.value==='' || !document.querySelector('input[type=password]')`)
		}
	}
}

// A real HTTP session must survive target-side cookie re-encryption, not just
// file copying. Uses disposable profiles and a loopback fixture only.
func TestLoginMigrationConfiguredKernel(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		cookie, err := r.Cookie("test_session")
		if err != nil || cookie.Value != "synthetic-session" {
			http.Error(w, "not logged in", 401)
			return
		}
		fmt.Fprint(w, "logged in")
	}))
	defer server.Close()
	source := t.TempDir()
	port := startLoginTestKernel(t, source, false)
	if err := cdpBrowserCall(port, "Storage.setCookies", map[string]any{"cookies": []map[string]any{
		{"name": "test_session", "value": "synthetic-session", "domain": "127.0.0.1", "path": "/", "httpOnly": true, "sameSite": "Lax"},
		{"name": "persistent", "value": "synthetic-persistent", "domain": "127.0.0.1", "path": "/", "expires": float64(time.Now().Add(time.Hour).Unix())},
		{"name": "partitioned_fixture", "value": "synthetic-partitioned", "domain": "example.invalid", "path": "/", "secure": true, "httpOnly": true, "sameSite": "None", "priority": "High", "partitionKey": map[string]any{"topLevelSite": "https://top.example.invalid", "hasCrossSiteAncestor": true}},
	}}); err != nil {
		t.Fatal(err)
	}
	state, err := capturePortableLoginState(port)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLocalLoginState(source, loginSnapshotFile, state); err != nil {
		t.Fatal(err)
	}
	envelope, err := createProfilePortableLoginEnvelope(source, "synthetic-migration-password")
	if err != nil {
		t.Fatal(err)
	}
	if envelope.PayloadType != "cookies-v2" {
		t.Fatal("not a cookie migration envelope")
	}
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "Local State"), []byte(`{"os_crypt":{"app_bound_encrypted_key":"synthetic-unusable-source-key"},"custom":{"keep":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreProfilePortableLoginEnvelope(target, "synthetic-migration-password", *envelope); err != nil {
		t.Fatal(err)
	}
	var localState map[string]any
	b, _ := os.ReadFile(filepath.Join(target, "Local State"))
	if json.Unmarshal(b, &localState) != nil || localState["custom"] == nil {
		t.Fatal("unrelated Local State structure changed")
	}
	targetPort := startLoginTestKernel(t, target, true)
	b, err = cdpGetEndpointBody(targetPort, "/json/list")
	if err != nil {
		t.Fatal(err)
	}
	var targets []cdpTarget
	if json.Unmarshal(b, &targets) != nil || len(targets) != 0 {
		t.Fatalf("startup gate opened pages before cookies: count=%d", len(targets))
	}
	pending, err := readLocalLoginState(target, loginRestoreFile)
	if err != nil {
		t.Fatal(err)
	}
	count, err := restorePortableCookies(targetPort, pending)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("restored cookie count=%d", count)
	}
	if requests.Load() != 0 {
		t.Fatal("fixture was contacted before restoration")
	}
	if err := createBrowserStartTarget(targetPort, server.URL); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, evalErr := cdpCall(targetPort, "Runtime.evaluate", map[string]any{"expression": "document.body?.innerText", "returnByValue": true})
		if evalErr == nil {
			if r, ok := result["result"].(map[string]any); ok && r["value"] == "logged in" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("restored session was not recognized by HTTP server")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
