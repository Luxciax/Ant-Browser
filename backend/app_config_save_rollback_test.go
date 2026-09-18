package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func newConfigSaveFailureApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	blockedRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := NewApp(blockedRoot)
	app.config = config.DefaultConfig()
	app.browserMgr = browser.NewManager(app.config, blockedRoot)
	return app
}

func TestAutomationSettingsRollbackMemoryWhenSaveFails(t *testing.T) {
	app := newConfigSaveFailureApp(t)
	previous := app.config.Automation
	if _, err := app.SaveAutomationSettings(!previous.Enabled, !previous.HeadlessDefault); err == nil {
		t.Fatal("SaveAutomationSettings should fail")
	}
	if app.config.Automation != previous {
		t.Fatalf("automation config was not rolled back: got=%+v want=%+v", app.config.Automation, previous)
	}

	if _, err := app.SaveAutomationRuntimeSettings("system", `C:\node\node.exe`); err == nil {
		t.Fatal("SaveAutomationRuntimeSettings should fail")
	}
	if app.config.Automation != previous {
		t.Fatalf("automation runtime config was not rolled back: got=%+v want=%+v", app.config.Automation, previous)
	}

	if _, err := app.SaveAutomationScriptPackageSettings(!previous.AllowTypeScriptBuild); err == nil {
		t.Fatal("SaveAutomationScriptPackageSettings should fail")
	}
	if app.config.Automation != previous {
		t.Fatalf("automation package config was not rolled back: got=%+v want=%+v", app.config.Automation, previous)
	}
}

func TestBrowserSettingsRollbackMemoryWhenSaveFails(t *testing.T) {
	app := newConfigSaveFailureApp(t)
	previous := app.config.Browser
	settings := app.GetBrowserSettings()
	settings.UserDataRoot = `D:\new-browser-data`
	settings.DefaultLaunchArgs = []string{"--disable-gpu"}
	settings.DefaultStartURLs = []string{"https://example.com"}
	if err := app.SaveBrowserSettings(settings); err == nil {
		t.Fatal("SaveBrowserSettings should fail")
	}
	if !reflect.DeepEqual(app.config.Browser, previous) {
		t.Fatalf("browser config was not rolled back")
	}
}

func TestProxyCheckSettingsRollbackMemoryWhenSaveFails(t *testing.T) {
	app := newConfigSaveFailureApp(t)
	previous := app.config.ProxyCheck
	settings := app.GetProxyCheckSettings()
	settings.BridgeStartTimeoutMs++
	if err := app.SaveProxyCheckSettings(settings); err == nil {
		t.Fatal("SaveProxyCheckSettings should fail")
	}
	if !reflect.DeepEqual(app.config.ProxyCheck, previous) {
		t.Fatalf("proxy check config was not rolled back")
	}
}

func TestBookmarkFallbackRollbackMemoryWhenSaveFails(t *testing.T) {
	app := newConfigSaveFailureApp(t)
	app.browserMgr.BookmarkDAO = nil
	previous := append([]BrowserBookmark(nil), app.config.Browser.DefaultBookmarks...)
	if err := app.BookmarkSave([]BrowserBookmark{{Name: "Example", URL: "https://example.com"}}); err == nil {
		t.Fatal("BookmarkSave should fail")
	}
	if !reflect.DeepEqual(app.config.Browser.DefaultBookmarks, previous) {
		t.Fatalf("bookmark config was not rolled back")
	}
}
