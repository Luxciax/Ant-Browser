package proxy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeProxyPreflightWithManagedOutput(t *testing.T) {
	for _, engine := range []string{"XRAY", "SINGBOX"} {
		t.Run(engine, func(t *testing.T) {
			binary := os.Getenv("ANT_BROWSER_TEST_" + engine)
			if binary == "" {
				t.Skip("set ANT_BROWSER_TEST_" + engine + " for native preflight acceptance")
			}
			dir := t.TempDir()
			cfg := filepath.Join(dir, "config.json")
			output := filepath.Join(dir, "preflight.log")
			content := `{"outbounds":[{"protocol":"freedom","tag":"direct"}]}`
			if engine == "SINGBOX" {
				content = `{"outbounds":[{"type":"direct","tag":"direct"}]}`
			}
			if err := os.WriteFile(cfg, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			run := func() error {
				if engine == "XRAY" {
					return (&XrayManager{}).testRuntimeConfigContext(context.Background(), binary, cfg, output)
				}
				return (&SingBoxManager{}).testRuntimeConfigContext(context.Background(), binary, cfg, output)
			}
			if err := run(); err != nil {
				t.Fatalf("valid preflight did not run: %v", err)
			}
			if err := os.WriteFile(cfg, []byte(`{"invalid":"fake","outbounds":[{"type":"nonexistent","protocol":"nonexistent"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := run(); err == nil || strings.Contains(err.Error(), "Stdout already set") {
				t.Fatalf("invalid config evidence missing: %v", err)
			}
			files, _ := filepath.Glob(filepath.Join(dir, "preflight-*.log"))
			if len(files) != 2 {
				t.Fatal("attempt output overwritten", files)
			}
		})
	}
}
