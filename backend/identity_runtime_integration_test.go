package backend

import (
	"ant-chrome/backend/internal/identity"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Explicit opt-in: uses only a disposable profile and never the user's profile.
func TestIdentityRuntimeConfiguredKernel(t *testing.T) {
	kernel := os.Getenv("ANT_IDENTITY_TEST_CHROME")
	if kernel == "" {
		t.Skip("set ANT_IDENTITY_TEST_CHROME to test the configured kernel")
	}
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(kernel, "--headless=new", "--no-first-run", "--no-default-browser-check", fmt.Sprintf("--user-data-dir=%s", t.TempDir()), fmt.Sprintf("--remote-debugging-port=%d", port), "about:blank")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cdpBrowserCall(port, "Browser.close", nil); _ = cmd.Wait() }()
	deadline := time.Now().Add(12 * time.Second)
	for {
		if _, err := cdpGetEndpointBody(port, "/json/version"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("isolated kernel did not expose CDP")
		}
		time.Sleep(100 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var warnings []string
	go identity.ObserveRuntime(ctx, port, identity.Identity{Locale: "ja-JP", Timezone: "Asia/Tokyo", Geo: &identity.Geo{Latitude: 35.68, Longitude: 139.76, Accuracy: 1000}}, func(name string, fields map[string]any) {
		if name == "identity.override_failed" {
			mu.Lock()
			warnings = append(warnings, fmt.Sprint(fields))
			mu.Unlock()
		}
	})
	read := func() (map[string]any, error) {
		r, err := cdpCall(port, "Runtime.evaluate", map[string]any{"expression": `JSON.stringify({locale:Intl.DateTimeFormat().resolvedOptions().locale,tz:Intl.DateTimeFormat().resolvedOptions().timeZone})`, "returnByValue": true})
		if err != nil {
			return nil, err
		}
		result, _ := r["result"].(map[string]any)
		value, _ := result["value"].(string)
		var out map[string]any
		err = json.Unmarshal([]byte(value), &out)
		return out, err
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		r, err := read()
		if err == nil && r["locale"] == "ja-JP" && r["tz"] == "Asia/Tokyo" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("page locale/timezone not applied: %v %v", r, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	popupID := ""
	for _, label := range []string{"new tab", "popup"} {
		created, err := cdpBrowserCallResult(port, "Target.createTarget", map[string]any{"url": "about:blank", "newWindow": label == "popup"})
		if err != nil {
			t.Fatal(err)
		}
		popupID, _ = created["targetId"].(string)
	}
	// /json ordering changes when multiple windows exist. Pin the actual popup
	// target so navigation, permissions and evaluation cannot hit different tabs.
	data, err := cdpGetEndpointBody(port, "/json/list")
	if err != nil {
		t.Fatal(err)
	}
	var targets []cdpTarget
	if err := json.Unmarshal(data, &targets); err != nil {
		t.Fatal(err)
	}
	popupWS := ""
	for _, target := range targets {
		if target.ID == popupID {
			popupWS = target.WebSocketDebuggerUrl
		}
	}
	conn, err := cdpDialWebSocket(popupWS)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sequence := 0
	popupCall := func(method string, params map[string]any) (map[string]any, error) {
		sequence++
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(cdpMessage{Id: sequence, Method: method, Params: params}); err != nil {
			return nil, err
		}
		for {
			var response cdpResponse
			if err := conn.ReadJSON(&response); err != nil {
				return nil, err
			}
			if response.Id != sequence {
				continue
			}
			if response.Error != nil {
				return nil, fmt.Errorf("%s: %s", method, response.Error.Message)
			}
			return response.Result, nil
		}
	}
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<!doctype html><title>isolated identity fixture</title>")
	}))
	defer page.Close()
	versionData, err := cdpGetEndpointBody(port, "/json/version")
	if err != nil {
		t.Fatal(err)
	}
	var version cdpBrowserVersion
	if err := json.Unmarshal(versionData, &version); err != nil {
		t.Fatal(err)
	}
	permissionConn, err := cdpDialWebSocket(version.WebSocketDebuggerUrl)
	if err != nil {
		t.Fatal(err)
	}
	defer permissionConn.Close()
	permissionConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// Permission overrides are owned by the CDP browser session. Keep it open;
	// a one-command helper disconnects immediately and resets its permissions.
	if err := permissionConn.WriteJSON(cdpMessage{Id: 1, Method: "Browser.grantPermissions", Params: map[string]any{"origin": page.URL, "permissions": []string{"geolocation"}}}); err != nil {
		t.Fatal(err)
	}
	var permissionResponse cdpResponse
	if err := permissionConn.ReadJSON(&permissionResponse); err != nil {
		t.Fatal(err)
	}
	if permissionResponse.Error != nil {
		t.Fatal(permissionResponse.Error.Message)
	}
	if _, err := popupCall("Page.navigate", map[string]any{"url": page.URL}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		result, err := popupCall("Runtime.evaluate", map[string]any{"expression": `new Promise(resolve=>navigator.geolocation.getCurrentPosition(p=>resolve(JSON.stringify({lat:p.coords.latitude,lon:p.coords.longitude})),e=>resolve(JSON.stringify({error:e.code,origin:location.origin})),{timeout:1000}))`, "awaitPromise": true, "returnByValue": true})
		if err != nil {
			t.Fatal(err)
		}
		remote, _ := result["result"].(map[string]any)
		value, _ := remote["value"].(string)
		var geo map[string]any
		json.Unmarshal([]byte(value), &geo)
		if geo["lat"] == 35.68 && geo["lon"] == 139.76 {
			break
		}
		if time.Now().After(deadline) {
			state, _ := popupCall("Runtime.evaluate", map[string]any{"expression": `navigator.permissions.query({name:'geolocation'}).then(p=>JSON.stringify({state:p.state,secure:window.isSecureContext,origin:location.origin}))`, "awaitPromise": true, "returnByValue": true})
			mu.Lock()
			copyWarnings := append([]string(nil), warnings...)
			mu.Unlock()
			t.Fatalf("popup geolocation not applied: %s; permission=%v; protocol=%v", value, state, copyWarnings)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Creating a worker exercises recursive auto-attach. Completion proves the
	// debugger wait was released; its locale must match the parent page.
	r, err := popupCall("Runtime.evaluate", map[string]any{"expression": `new Promise(resolve=>{const worker=new Worker(URL.createObjectURL(new Blob(['postMessage({locale:Intl.DateTimeFormat().resolvedOptions().locale,tz:Intl.DateTimeFormat().resolvedOptions().timeZone})'],{type:'text/javascript'})));worker.onmessage=e=>{worker.terminate();resolve(JSON.stringify(e.data))};setTimeout(()=>resolve('timeout'),3000)})`, "awaitPromise": true, "returnByValue": true})
	if err != nil {
		t.Fatal(err)
	}
	result, _ := r["result"].(map[string]any)
	value, _ := result["value"].(string)
	if value == "timeout" {
		t.Fatal("worker left waiting for debugger")
	}
	var worker map[string]any
	if err := json.Unmarshal([]byte(value), &worker); err != nil {
		t.Fatalf("worker result: %s", value)
	}
	if worker["locale"] != "ja-JP" || worker["tz"] != "Asia/Tokyo" {
		t.Errorf("worker inconsistency: %s", value)
	}
	mu.Lock()
	defer mu.Unlock()
	t.Logf("kernel=%s, worker=%v, protocol warnings=%v", kernel, worker, warnings)
}

func TestIdentityUIConfiguredKernel(t *testing.T) {
	kernel, uiURL := os.Getenv("ANT_IDENTITY_TEST_CHROME"), os.Getenv("ANT_IDENTITY_UI_URL")
	if kernel == "" || uiURL == "" {
		t.Skip("requires configured kernel and local UI fixture URL")
	}
	port, err := nextAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(kernel, "--headless=new", "--no-first-run", fmt.Sprintf("--user-data-dir=%s", t.TempDir()), fmt.Sprintf("--remote-debugging-port=%d", port), "about:blank")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cdpBrowserCall(port, "Browser.close", nil); _ = cmd.Wait() }()
	deadline := time.Now().Add(12 * time.Second)
	for {
		if _, err := cdpGetEndpointBody(port, "/json/version"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			t.Fatal("UI kernel did not expose CDP")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := cdpCall(port, "Page.navigate", map[string]any{"url": uiURL}); err != nil {
		t.Fatal(err)
	}
	eval := func(expression string) string {
		r, err := cdpCall(port, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true})
		if err != nil {
			t.Fatal(err)
		}
		if r["exceptionDetails"] != nil {
			t.Fatalf("fixture JS exception: %v", r["exceptionDetails"])
		}
		result, _ := r["result"].(map[string]any)
		return fmt.Sprint(result["value"])
	}
	deadline = time.Now().Add(10 * time.Second)
	for eval(`document.querySelectorAll('#fish-identity-template option').length`) != "13" {
		if time.Now().After(deadline) {
			t.Fatal("fixture did not render templates: " + eval(`document.body.innerText.slice(0,500)`))
		}
		time.Sleep(100 * time.Millisecond)
	}
	eval(`document.querySelector('#fish-identity-template').value='linux-8-1920x1080';document.querySelector('#fish-identity-template').dispatchEvent(new Event('change',{bubbles:true}));true`)
	eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='应用画像').click();true`)
	deadline = time.Now().Add(3 * time.Second)
	for eval(`document.querySelector('#args').textContent.includes('--fingerprint-platform=linux')`) != "true" {
		if time.Now().After(deadline) {
			t.Fatal("template control did not apply")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if eval(`document.querySelector('#args').textContent.includes('--fingerprint=42')`) != "true" {
		t.Fatal("UI changed seed")
	}
	eval(`document.querySelector('summary').click();true`)
	eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='应用定位').click();true`)
	if eval(`document.querySelector('[role=alert]')?.textContent.includes('请填写有效纬度')`) != "true" {
		t.Fatal("geo validation is not visible")
	}
	for _, viewport := range []struct {
		name  string
		width int
	}{{"desktop", 1440}, {"narrow", 800}} {
		if _, err := cdpCall(port, "Emulation.setDeviceMetricsOverride", map[string]any{"width": viewport.width, "height": 1000, "deviceScaleFactor": 1, "mobile": false}); err != nil {
			t.Fatal(err)
		}
		if eval(`document.querySelector('.fish-identity-panel').scrollWidth<=document.querySelector('.fish-identity-panel').clientWidth`) != "true" {
			t.Fatal("identity panel overflows at " + viewport.name)
		}
		r, err := cdpCall(port, "Page.captureScreenshot", map[string]any{"format": "png"})
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(r["data"].(string))
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join("..", ".impeccable", "review")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, viewport.name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cdpCall(port, "Page.navigate", map[string]any{"url": uiURL + "?templates=fail"}); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for eval(`document.body.innerText.includes('fixture template failure')`) != "true" {
		if time.Now().After(deadline) {
			t.Fatal("template error not visible")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if eval(`Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='应用定位').disabled`) != "false" {
		t.Fatal("geolocation incorrectly depends on template loading")
	}
	t.Log("UI template, seed preservation, visible validation, independent geolocation, desktop and narrow screenshots passed; binding is simulated")
}
