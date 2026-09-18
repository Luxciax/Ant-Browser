package browser

import (
	"os"
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestStoreExtensionPackageKeepsVersionsSeparate(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(config.DefaultConfig(), root)
	extensionID := "abcdefghijklmnopabcdefghijklmnop"

	firstPath, firstHash, err := manager.storeExtensionPackage(extensionID, "1.0.0", []byte("Cr24-version-one"))
	if err != nil {
		t.Fatalf("store first version: %v", err)
	}
	secondPath, secondHash, err := manager.storeExtensionPackage(extensionID, "2.0.0", []byte("Cr24-version-two"))
	if err != nil {
		t.Fatalf("store second version: %v", err)
	}
	if sameExtensionPath(firstPath, secondPath) {
		t.Fatalf("versioned packages share path: %q", firstPath)
	}
	if firstHash == secondHash {
		t.Fatal("different package versions unexpectedly share a hash")
	}
	for _, item := range []struct {
		path    string
		version string
	}{
		{firstPath, "1.0.0"},
		{secondPath, "2.0.0"},
	} {
		want := filepath.Join("packages", extensionID, item.version, "extension.crx")
		if !stringsHasPathSuffix(item.path, want) {
			t.Fatalf("package path = %q, want suffix %q", item.path, want)
		}
		if _, err := os.Stat(item.path); err != nil {
			t.Fatalf("stored package missing at %q: %v", item.path, err)
		}
	}
}

func TestExtensionPackageVersionSegmentIsContained(t *testing.T) {
	for _, version := range []string{"../escape", `..\\escape`, "1.0.0/../../bad"} {
		segment := extensionPackageVersionSegment(version)
		if segment == "" || segment == "." || segment == ".." || filepath.IsAbs(segment) || filepath.Clean(segment) != segment {
			t.Fatalf("unsafe segment %q produced from %q", segment, version)
		}
		if filepath.Dir(segment) != "." {
			t.Fatalf("version %q produced nested segment %q", version, segment)
		}
	}
}

func stringsHasPathSuffix(path string, suffix string) bool {
	path = filepath.Clean(path)
	suffix = filepath.Clean(suffix)
	if len(path) < len(suffix) {
		return false
	}
	return sameExtensionPath(path[len(path)-len(suffix):], suffix)
}
