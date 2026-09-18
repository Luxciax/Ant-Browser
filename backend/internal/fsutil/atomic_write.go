package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to a temporary file in the destination
// directory, flushes it, and then atomically replaces the destination.
// Keeping the staging file on the same volume is required for atomic rename.
func AtomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	closed := false
	committed := false
	defer func() {
		if !closed {
			_ = temporary.Close()
		}
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("set staging file mode: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		closed = true
		return fmt.Errorf("close staging file: %w", err)
	}
	closed = true

	if err := ReplaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}
	committed = true
	return nil
}
