//go:build !windows

package fsutil

import "os"

// ReplaceFile atomically replaces targetPath with sourcePath on POSIX systems
// when both paths are on the same filesystem.
func ReplaceFile(sourcePath, targetPath string) error {
	return os.Rename(sourcePath, targetPath)
}
