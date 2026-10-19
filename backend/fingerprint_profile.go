package backend

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxFingerprintProfileBytes = 5 * 1024 * 1024

type FingerprintProfileImportResult struct {
	FilePath string `json:"filePath"`
}

func readFingerprintProfile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取设备 Profile: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxFingerprintProfileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxFingerprintProfileBytes {
		return nil, fmt.Errorf("设备 Profile JSON 超过 5 MiB")
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, fmt.Errorf("设备 Profile 必须是 JSON 对象")
	}
	return raw, nil
}

// Encodes the raw JSON exactly as the SDK's gzipSync(raw, {level: 9}).toString('base64').
func encodeFingerprintProfile(path string) (string, error) {
	raw, err := readFingerprintProfile(path)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err = writer.Write(raw); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	payload := base64.StdEncoding.EncodeToString(buffer.Bytes())
	if len(payload) > 24000 {
		return "", fmt.Errorf("设备 Profile 编码后超过安全命令行长度，请缩小采集 JSON")
	}
	return payload, nil
}

func (a *App) importFingerprintProfileFile(sourcePath string) (*FingerprintProfileImportResult, error) {
	raw, err := readFingerprintProfile(sourcePath)
	if err != nil {
		return nil, err
	}
	if _, err = encodeFingerprintProfile(sourcePath); err != nil {
		return nil, err
	}
	directory := a.resolveAppPath(filepath.Join("data", "fingerprint-profiles"))
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	destination := filepath.Join(directory, fmt.Sprintf("%x.json", sha256.Sum256(raw)))
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return &FingerprintProfileImportResult{FilePath: destination}, nil
	}
	if err != nil {
		return nil, err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return &FingerprintProfileImportResult{FilePath: destination}, nil
}

func (a *App) BrowserFingerprintProfileImport() (*FingerprintProfileImportResult, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("文件选择器未初始化")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "导入真实设备 Fingerprint Profile", Filters: []wailsruntime.FileFilter{{DisplayName: "Fingerprint Profile JSON", Pattern: "*.json"}}})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	return a.importFingerprintProfileFile(path)
}

func prepareClearcoteProfileArgs(rawArgs, launchArgs []string) ([]string, error) {
	stored, err := readClearcoteStoredConfig(rawArgs)
	if err != nil {
		return nil, err
	}
	if stored.Mode == "auto" {
		return launchArgs, nil
	}
	if stored.FingerprintProfile == nil || stored.FingerprintProfile.FilePath == "" {
		if stored.Mode == "profile" {
			return nil, fmt.Errorf("请选择真实设备 Profile JSON")
		}
		return launchArgs, nil
	}
	if stored.FingerprintProfile.Source != "file" {
		return nil, fmt.Errorf("不支持的设备 Profile 来源")
	}
	payload, err := encodeFingerprintProfile(stored.FingerprintProfile.FilePath)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(launchArgs)+1)
	for _, arg := range launchArgs {
		if browserFingerprintArgKey(arg) != "--fingerprint-profile" {
			result = append(result, arg)
		}
	}
	return append(result, "--fingerprint-profile="+payload), nil
}
