package secretstore

import (
	"os"
	"runtime"
	"testing"
)

func TestSecretStoreRoundTripAndDelete(t *testing.T) {
	if runtime.GOOS != "windows" {
		old, had := os.LookupEnv(envMasterKeyName)
		if err := os.Setenv(envMasterKeyName, "phase5-test-secret-key-with-sufficient-entropy"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if had { _ = os.Setenv(envMasterKeyName, old) } else { _ = os.Unsetenv(envMasterKeyName) }
		}()
	}

	store, err := New(t.TempDir() + string(os.PathSeparator) + "secrets.json")
	if err != nil { t.Fatal(err) }

	ref, err := store.Set("", "test", "top-secret-value")
	if err != nil { t.Fatalf("Set: %v", err) }
	if !IsRef(ref) { t.Fatalf("expected secret ref, got %q", ref) }

	value, err := store.Get(ref)
	if err != nil { t.Fatalf("Get: %v", err) }
	if value != "top-secret-value" { t.Fatalf("unexpected value %q", value) }

	if err := store.Delete(ref); err != nil { t.Fatalf("Delete: %v", err) }
	if _, err := store.Get(ref); err == nil { t.Fatal("deleted secret should not resolve") }
}
