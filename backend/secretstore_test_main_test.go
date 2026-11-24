package backend

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	const key = "ANT_BROWSER_SECRET_KEY"
	_, had := os.LookupEnv(key)
	if !had {
		_ = os.Setenv(key, "ant-browser-tests-local-secret-key-2026")
	}
	code := m.Run()
	if !had {
		_ = os.Unsetenv(key)
	}
	os.Exit(code)
}
