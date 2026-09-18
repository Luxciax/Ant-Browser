package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserInstanceGetTabsReadsRealCDPTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/list" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[\n" +
			"{\"id\":\"page-1\",\"type\":\"page\",\"title\":\"First\",\"url\":\"https://first.example/\"},\n" +
			"{\"id\":\"worker-1\",\"type\":\"service_worker\",\"title\":\"Worker\",\"url\":\"https://worker.example/\"},\n" +
			"{\"id\":\"page-2\",\"type\":\"page\",\"title\":\"Second\",\"url\":\"about:blank\"}\n" +
			"]"))
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	root := t.TempDir()
	cfg := config.DefaultConfig()
	manager := browser.NewManager(cfg, root)
	manager.Profiles["profile-tabs"] = &browser.Profile{
		ProfileId:    "profile-tabs",
		RuntimeState: browser.RuntimeRunning,
		Running:      true,
		DebugReady:   true,
		DebugPort:    port,
	}
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = manager

	tabs, err := app.BrowserInstanceGetTabs("profile-tabs")
	if err != nil {
		t.Fatalf("BrowserInstanceGetTabs returned error: %v", err)
	}
	if len(tabs) != 2 {
		t.Fatalf("tab count = %d, want 2: %#v", len(tabs), tabs)
	}
	if tabs[0].TabId != "page-1" || tabs[0].Title != "First" || tabs[0].Url != "https://first.example/" {
		t.Fatalf("first tab = %#v", tabs[0])
	}
	if tabs[1].TabId != "page-2" || tabs[1].Title != "Second" || tabs[1].Url != "about:blank" {
		t.Fatalf("second tab = %#v", tabs[1])
	}
}

func TestBrowserInstanceGetTabsRejectsStartingProfile(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	manager := browser.NewManager(cfg, root)
	manager.Profiles["profile-starting"] = &browser.Profile{
		ProfileId:    "profile-starting",
		RuntimeState: browser.RuntimeStarting,
		DebugPort:    9222,
		DebugReady:   true,
	}
	app := NewApp(root)
	app.config = cfg
	app.browserMgr = manager

	if _, err := app.BrowserInstanceGetTabs("profile-starting"); err == nil {
		t.Fatal("starting profile unexpectedly exposed tabs")
	}
}
