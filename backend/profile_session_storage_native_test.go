package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionStorageBeforeFirstScriptConfiguredKernel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><script>document.documentElement.dataset.initial=sessionStorage.getItem('synthetic_auth')||'missing';</script></head><body>synthetic session fixture</body></html>`)
	}))
	defer server.Close()
	source := startLoginTestKernel(t, t.TempDir(), false)
	urls := []string{server.URL + "/?slot=a", server.URL + "/?slot=b"}
	for i, raw := range urls {
		created, err := cdpBrowserCallResult(source, "Target.createTarget", map[string]any{"url": raw})
		if err != nil {
			t.Fatal(err)
		}
		id, _ := created["targetId"].(string)
		target, err := resolveLoginPageTarget(source, id)
		if err != nil {
			t.Fatal(err)
		}
		page, err := dialLoginPage(target.WebSocketDebuggerUrl)
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			result, err := page.call("Runtime.evaluate", map[string]any{"expression": fmt.Sprintf(`location.origin===%q&&document.readyState==='complete'`, server.URL), "returnByValue": true})
			r, _ := result["result"].(map[string]any)
			if err == nil && r["value"] == true {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("source page not ready")
			}
			time.Sleep(25 * time.Millisecond)
		}
		if _, err := page.call("Runtime.evaluate", map[string]any{"expression": fmt.Sprintf(`sessionStorage.setItem('synthetic_auth','synthetic-tab-%d')`, i), "returnByValue": true}); err != nil {
			t.Fatal(err)
		}
		page.conn.Close()
	}
	state, err := capturePortableLoginState(source)
	if err != nil || len(state.Pages) != 2 || state.SkippedStorage != 0 {
		t.Fatalf("page snapshots=%d skipped=%d err=%v", len(state.Pages), state.SkippedStorage, err)
	}
	targetPort := startLoginTestKernel(t, t.TempDir(), true)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := openLoginRestoreTargets(ctx, targetPort, state, urls); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for i, raw := range urls {
		body, err := cdpGetEndpointBody(targetPort, "/json/list")
		if err != nil {
			t.Fatal(err)
		}
		var targets []cdpTarget
		if err := json.Unmarshal(body, &targets); err != nil {
			t.Fatal(err)
		}
		for _, target := range targets {
			if target.URL == raw {
				matched++
				page, err := dialLoginPage(target.WebSocketDebuggerUrl)
				if err != nil {
					t.Fatal(err)
				}
				result, err := page.call("Runtime.evaluate", map[string]any{"expression": "document.documentElement.dataset.initial", "returnByValue": true})
				r, _ := result["result"].(map[string]any)
				if err != nil || r["value"] != fmt.Sprintf("synthetic-tab-%d", i) {
					t.Fatal("first website script did not see its own tab's session")
				}
				_, err = page.call("Runtime.evaluate", map[string]any{"expression": `sessionStorage.setItem('synthetic_auth','synthetic-updated');location.reload();true`, "returnByValue": true})
				if err != nil {
					t.Fatal(err)
				}
				deadline := time.Now().Add(5 * time.Second)
				for {
					result, err := page.call("Runtime.evaluate", map[string]any{"expression": "document.documentElement.dataset.initial", "returnByValue": true})
					r, _ := result["result"].(map[string]any)
					if err == nil && r["value"] == "synthetic-updated" {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("later navigation replayed stale session snapshot")
					}
					time.Sleep(25 * time.Millisecond)
				}
				page.conn.Close()
			}
		}
	}
	if matched != 2 {
		t.Fatal("restored tabs not found")
	}
}
