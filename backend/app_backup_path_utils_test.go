package backend

import (
	"path/filepath"
	"testing"
)

func TestBackupPathWithinUsesPathBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backup")
	inside := filepath.Join(root, "profiles", "profile-a")
	siblingPrefix := root + "-old"
	escape := filepath.Join(root, "..", "outside")

	if !backupPathWithin(root, root) {
		t.Fatal("root should be considered within itself")
	}
	if !backupPathWithin(inside, root) {
		t.Fatalf("inside path %q was rejected", inside)
	}
	if backupPathWithin(siblingPrefix, root) {
		t.Fatalf("same-prefix sibling %q was treated as inside %q", siblingPrefix, root)
	}
	if backupPathWithin(escape, root) {
		t.Fatalf("parent escape %q was treated as inside %q", escape, root)
	}
}
