package apppath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectForOSDoesNotCacheTransientDetachedResult(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app-root")

	first := detectForOS(root, "linux")
	if !first.detached {
		t.Fatalf("first detached = false, want true for missing root")
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create app root: %v", err)
	}
	second := detectForOS(root, "linux")
	if second.detached {
		t.Fatalf("second detached = true, want fresh writable probe result")
	}
	if second.stateRoot != second.installRoot {
		t.Fatalf("stateRoot = %q, installRoot = %q, want same root after recovery", second.stateRoot, second.installRoot)
	}
}
