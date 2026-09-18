package backend

import (
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveProxyCoreBinaryPathRollsBackMemoryWhenConfigSaveFails(t *testing.T) {
	root := t.TempDir()
	blockedRoot := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := NewApp(blockedRoot)
	app.config = config.DefaultConfig()
	app.config.Browser.XrayBinaryPath = `C:\old\xray.exe`

	err := app.saveProxyCoreBinaryPath(proxyCoreSpec{ConfigKey: "xray"}, `D:\new\xray.exe`)
	if err == nil {
		t.Fatal("saveProxyCoreBinaryPath should fail when config path cannot be created")
	}
	if app.config.Browser.XrayBinaryPath != `C:\old\xray.exe` {
		t.Fatalf("XrayBinaryPath = %q, want rollback to old path", app.config.Browser.XrayBinaryPath)
	}
}
