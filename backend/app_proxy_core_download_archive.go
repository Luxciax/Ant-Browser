package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func extractProxyCoreArchive(archivePath string, targetDir string, binaryBase string, targetOS string) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZipArchive(archivePath, targetDir)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGzArchive(archivePath, targetDir)
	}
	if strings.HasSuffix(lower, ".gz") {
		return extractGzipBinary(archivePath, filepath.Join(targetDir, proxyCoreBinaryName(binaryBase, targetOS)))
	}
	return fmt.Errorf("不支持的压缩格式: %s", filepath.Base(archivePath))
}

func extractGzipBinary(archivePath string, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, gz)
	return err
}

func extractZipArchive(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if err := writeArchiveFile(targetDir, file.Name, file.FileInfo().Mode(), file.FileInfo().IsDir(), func() (io.ReadCloser, error) { return file.Open() }); err != nil {
			return err
		}
	}
	return nil
}

func extractTarGzArchive(archivePath string, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		mode := os.FileMode(header.Mode)
		isDir := header.FileInfo().IsDir()
		if header.Typeflag == tar.TypeDir {
			isDir = true
		}
		if err := writeArchiveFile(targetDir, header.Name, mode, isDir, func() (io.ReadCloser, error) { return io.NopCloser(tr), nil }); err != nil {
			return err
		}
	}
}

func writeArchiveFile(targetDir string, name string, mode os.FileMode, isDir bool, open func() (io.ReadCloser, error)) error {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) || filepath.IsAbs(cleanName) {
		return fmt.Errorf("压缩包包含非法路径: %s", name)
	}
	dest := filepath.Join(targetDir, cleanName)
	if isDir {
		return os.MkdirAll(dest, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func findProxyCoreBinary(root string, binaryBase string, targetOS string) (string, error) {
	names := []string{proxyCoreBinaryName(binaryBase, targetOS), binaryBase}
	var matches []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		for _, name := range names {
			if proxyCoreBinaryNameMatches(base, strings.ToLower(name), binaryBase, targetOS) {
				matches = append(matches, path)
				break
			}
		}
		return nil
	})
	if len(matches) == 0 {
		return "", fmt.Errorf("解压后未找到 %s 可执行文件", binaryBase)
	}
	sort.Strings(matches)
	return matches[0], nil
}

func proxyCoreBinaryNameMatches(base string, expected string, binaryBase string, targetOS string) bool {
	if base == expected {
		return true
	}
	baseNoExt := strings.TrimSuffix(base, ".exe")
	expectedNoExt := strings.TrimSuffix(expected, ".exe")
	if baseNoExt == expectedNoExt {
		return true
	}
	if binaryBase == "mihomo" && strings.HasPrefix(baseNoExt, "mihomo-") {
		return targetOS != "windows" || strings.HasSuffix(base, ".exe")
	}
	return false
}

func normalizeInstalledProxyCoreBinary(binaryPath string, installDir string, binaryBase string, targetOS string) (string, error) {
	standardPath := filepath.Join(installDir, proxyCoreBinaryName(binaryBase, targetOS))
	if sameCleanPath(binaryPath, standardPath) {
		return binaryPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(standardPath), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(standardPath); err == nil {
		if err := os.Remove(standardPath); err != nil {
			return "", err
		}
	}
	if err := os.Rename(binaryPath, standardPath); err != nil {
		return "", err
	}
	return standardPath, nil
}

type proxyCoreDirectorySwap struct {
	installDir  string
	rollbackDir string
	hadOriginal bool
}

func stageProxyCoreDirectory(stagingDir string, installDir string) (proxyCoreDirectorySwap, error) {
	swap := proxyCoreDirectorySwap{installDir: filepath.Clean(installDir)}
	stagingDir = filepath.Clean(stagingDir)
	if stagingDir == swap.installDir {
		return proxyCoreDirectorySwap{}, fmt.Errorf("代理内核暂存目录不能与安装目录相同")
	}
	if info, err := os.Stat(stagingDir); err != nil {
		return proxyCoreDirectorySwap{}, fmt.Errorf("检查代理内核暂存目录失败: %w", err)
	} else if !info.IsDir() {
		return proxyCoreDirectorySwap{}, fmt.Errorf("代理内核暂存路径不是目录: %s", stagingDir)
	}

	swap.rollbackDir = fmt.Sprintf("%s.rollback-%d", swap.installDir, time.Now().UnixNano())
	if _, err := os.Stat(swap.installDir); err == nil {
		swap.hadOriginal = true
		if err := os.Rename(swap.installDir, swap.rollbackDir); err != nil {
			return proxyCoreDirectorySwap{}, fmt.Errorf("暂存旧代理内核失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return proxyCoreDirectorySwap{}, fmt.Errorf("检查旧代理内核失败: %w", err)
	}

	if err := os.Rename(stagingDir, swap.installDir); err != nil {
		if swap.hadOriginal {
			if restoreErr := os.Rename(swap.rollbackDir, swap.installDir); restoreErr != nil {
				return proxyCoreDirectorySwap{}, fmt.Errorf("提交新代理内核失败: %w；恢复旧代理内核失败: %v", err, restoreErr)
			}
		}
		return proxyCoreDirectorySwap{}, fmt.Errorf("提交新代理内核失败: %w", err)
	}
	return swap, nil
}

func (s proxyCoreDirectorySwap) rollback() error {
	if err := os.RemoveAll(s.installDir); err != nil {
		return fmt.Errorf("清理新代理内核失败: %w", err)
	}
	if !s.hadOriginal {
		return nil
	}
	if err := os.Rename(s.rollbackDir, s.installDir); err != nil {
		return fmt.Errorf("恢复旧代理内核失败: %w", err)
	}
	return nil
}

func (s proxyCoreDirectorySwap) finish() {
	if s.hadOriginal {
		_ = os.RemoveAll(s.rollbackDir)
	}
}
