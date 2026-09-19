package backend

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func backupSQLiteFile(path string) (bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil {
		return false, err
	} else if info.IsDir() {
		return false, nil
	}
	var header [16]byte
	_, err = io.ReadFull(f, header[:])
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return string(header[:]) == "SQLite format 3\x00", nil
}

func backupSQLiteSidecar(path, target string) (bool, error) {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if strings.HasSuffix(path, suffix) {
			for _, base := range []string{strings.TrimSuffix(path, suffix), strings.TrimSuffix(target, suffix)} {
				ok, err := backupSQLiteFile(base)
				if err != nil {
					return false, err
				}
				if ok {
					return true, nil
				}
			}
			wal := strings.TrimSuffix(path, suffix) + "-wal"
			f, err := os.Open(wal)
			if os.IsNotExist(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			defer f.Close()
			var magic [4]byte
			if _, err := io.ReadFull(f, magic[:]); err != nil {
				return false, nil
			}
			value := binary.BigEndian.Uint32(magic[:])
			return value == 0x377f0682 || value == 0x377f0683, nil
		}
	}
	return false, nil
}

// A destination database and all its sidecars are one conflict unit. A new
// database is exported through SQLite so WAL pages cannot be mixed with a
// different base file, and no partial pair is published on copy failure.
func backupImportSQLite(source, target string, stats *backupMergeStats) error {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(target + suffix); err == nil {
			stats.Conflicts++
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), ".sqlite-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	destination := filepath.Join(staging, "database")
	path := filepath.ToSlash(source)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	_, exportErr := db.Exec("VACUUM INTO ?", destination)
	closeErr := db.Close()
	if exportErr != nil {
		return exportErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(destination, target); err != nil {
		return err
	}
	stats.Imported++
	return nil
}

func backupLevelDBDir(dir string) (bool, error) {
	current, err := os.ReadFile(filepath.Join(dir, "CURRENT"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	manifest := strings.TrimSpace(string(current))
	if !strings.HasPrefix(manifest, "MANIFEST-") || filepath.Base(manifest) != manifest {
		return false, nil
	}
	info, err := os.Stat(filepath.Join(dir, manifest))
	if err != nil {
		return false, fmt.Errorf("备份中的 LevelDB manifest 不完整: %w", err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("备份中的 LevelDB manifest 不是文件")
	}
	return true, nil
}

func backupImportLevelDB(source, target string, stats *backupMergeStats) error {
	if _, err := os.Lstat(target); err == nil {
		stats.Conflicts++
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(target), ".leveldb-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	count := 0
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fs.ErrInvalid
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(staging, rel)
		if entry.IsDir() {
			return os.MkdirAll(to, 0755)
		}
		if err := backupCopyFile(path, to); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		return err
	}
	stats.Imported += count
	return nil
}
