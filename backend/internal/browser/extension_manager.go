package browser

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	extensionDownloadTimeout = 90 * time.Second
	extensionMaxPackageBytes = 128 << 20
	extensionsRootDir        = "extensions"
	profileExtensionsDir     = "AntiBrowserExtensions"
)

func ExtensionDownloadTimeout() time.Duration {
	return extensionDownloadTimeout
}

var extensionIDPattern = regexp.MustCompile(`^[a-p]{32}$`)

type extensionManifest struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description"`
	ShortName     string            `json:"short_name"`
	DefaultLocale string            `json:"default_locale"`
	Icons         map[string]string `json:"icons"`
	Action        map[string]any    `json:"action"`
	BrowserAction map[string]any    `json:"browser_action"`
}

func NormalizeExtensionID(value string) string {
	trimmed := strings.TrimSpace(value)
	if parsed := extractExtensionIDFromURL(trimmed); parsed != "" {
		trimmed = parsed
	}
	trimmed = strings.ToLower(strings.Trim(trimmed, "/#?& "))
	if extensionIDPattern.MatchString(trimmed) {
		return trimmed
	}
	return ""
}

func BuildChromeWebStoreURL(extensionID string) string {
	normalizedID := NormalizeExtensionID(extensionID)
	if normalizedID == "" {
		return ""
	}
	return "https://chromewebstore.google.com/detail/" + normalizedID
}

func BuildChromeExtensionDownloadURL(extensionID string) string {
	normalizedID := NormalizeExtensionID(extensionID)
	if normalizedID == "" {
		return ""
	}
	return "https://clients2.google.com/service/update2/crx?response=redirect&prodversion=120.0.0.0&acceptformat=crx2,crx3&x=id%3D" + normalizedID + "%26installsource%3Dondemand%26uc"
}

func (m *Manager) LookupExtension(query string) (ExtensionLookupResult, error) {
	return m.LookupExtensionWithHTTPClient(query, nil)
}

func (m *Manager) LookupExtensionWithHTTPClient(query string, client *http.Client) (ExtensionLookupResult, error) {
	extensionID := NormalizeExtensionID(query)
	if extensionID == "" {
		return ExtensionLookupResult{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	result := ExtensionLookupResult{
		ExtensionID: extensionID,
		Name:        extensionID,
		StoreURL:    BuildChromeWebStoreURL(extensionID),
		Installable: true,
		Message:     "已识别插件 ID，可下载安装",
	}
	data, err := downloadChromeExtensionCRX(context.Background(), extensionID, client)
	if err != nil {
		result.Message = "已识别插件 ID，但暂时无法读取商店元信息: " + err.Error()
		return result, nil
	}
	zipData, err := normalizeExtensionArchiveData(data)
	if err != nil {
		result.Message = "已识别插件 ID，但插件包格式无法解析: " + err.Error()
		return result, nil
	}
	manifestData, err := readExtensionManifestFromZip(zipData)
	if err != nil {
		result.Message = "已识别插件 ID，但 manifest 无法解析: " + err.Error()
		return result, nil
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		result.Message = "已识别插件 ID，但 manifest 无法解析: " + err.Error()
		return result, nil
	}
	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	result.Name = resolveExtensionName(manifest, extensionID)
	result.Name = resolveExtensionMessage(result.Name, localeMessages)
	result.Version = strings.TrimSpace(manifest.Version)
	result.Description = resolveExtensionDescription(manifest, localeMessages)
	result.Message = "已读取插件信息，可下载安装"
	return ExtensionLookupResult{
		ExtensionID: result.ExtensionID,
		Name:        result.Name,
		Version:     result.Version,
		Description: result.Description,
		StoreURL:    result.StoreURL,
		Installable: result.Installable,
		Message:     result.Message,
	}, nil
}

func (m *Manager) InstallExtensionFromWebStore(ctx context.Context, query string) (Extension, error) {
	return m.InstallExtensionFromWebStoreWithHTTPClient(ctx, query, nil)
}

func (m *Manager) InstallExtensionFromWebStoreWithHTTPClient(ctx context.Context, query string, client *http.Client) (Extension, error) {
	extensionID := NormalizeExtensionID(query)
	if extensionID == "" {
		return Extension{}, fmt.Errorf("请输入 Chrome 插件 ID 或 Chrome Web Store 链接")
	}
	data, err := downloadChromeExtensionCRX(ctx, extensionID, client)
	if err != nil {
		return Extension{}, err
	}
	return m.InstallExtensionPackageBytes(extensionID, BuildChromeWebStoreURL(extensionID), data)
}

func (m *Manager) InstallExtensionPackageBytes(extensionID string, sourceURL string, data []byte) (Extension, error) {
	if len(data) == 0 {
		return Extension{}, fmt.Errorf("插件包为空")
	}
	if len(data) > extensionMaxPackageBytes {
		return Extension{}, fmt.Errorf("插件包超过限制")
	}
	zipData, err := normalizeExtensionArchiveData(data)
	if err != nil {
		return Extension{}, err
	}
	manifestData, err := readExtensionManifestFromZip(zipData)
	if err != nil {
		return Extension{}, err
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		return Extension{}, err
	}

	resolvedID := NormalizeExtensionID(extensionID)
	if resolvedID == "" {
		resolvedID = extensionIDFromManifest(manifestData)
	}
	if resolvedID == "" {
		return Extension{}, fmt.Errorf("无法识别插件 ID")
	}

	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), resolvedID)
	if err := replaceExtensionDirFromZip(zipData, installDir); err != nil {
		return Extension{}, err
	}

	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	manifestJSON := string(manifestData)
	extension := Extension{
		ExtensionID:  resolvedID,
		Name:         resolveExtensionMessage(resolveExtensionName(manifest, resolvedID), localeMessages),
		Version:      strings.TrimSpace(manifest.Version),
		Description:  resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:  readExtensionIconDataURLFromZip(zipData, manifest),
		ManifestJSON: manifestJSON,
		SourceURL:    strings.TrimSpace(sourceURL),
		InstallDir:   installDir,
		Enabled:      true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		stored, err := m.ExtensionDAO.Get(resolvedID)
		if err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

func (m *Manager) InstallExtensionPackageFile(path string) (Extension, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return Extension{}, fmt.Errorf("插件文件路径不能为空")
	}
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return Extension{}, fmt.Errorf("读取插件文件失败: %w", err)
	}
	return m.InstallExtensionPackageBytes("", normalizedPath, data)
}

func (m *Manager) InstallExtensionDirectory(sourceDir string) (Extension, error) {
	normalizedDir := strings.TrimSpace(sourceDir)
	if normalizedDir == "" {
		return Extension{}, fmt.Errorf("插件目录不能为空")
	}
	manifestPath := filepath.Join(normalizedDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return Extension{}, fmt.Errorf("插件目录缺少 manifest.json: %w", err)
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		return Extension{}, err
	}
	extensionID := extensionIDFromManifest(manifestData)
	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), extensionID)
	if err := copyExtensionDirectory(normalizedDir, installDir); err != nil {
		return Extension{}, err
	}
	localeMessages := readExtensionLocaleMessagesFromDir(normalizedDir, manifest)
	extension := Extension{
		ExtensionID:  extensionID,
		Name:         resolveExtensionMessage(resolveExtensionName(manifest, extensionID), localeMessages),
		Version:      strings.TrimSpace(manifest.Version),
		Description:  resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:  readExtensionIconDataURLFromDir(normalizedDir, manifest),
		ManifestJSON: string(manifestData),
		SourceURL:    normalizedDir,
		InstallDir:   installDir,
		Enabled:      true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, err
		}
		if stored, err := m.ExtensionDAO.Get(extensionID); err == nil {
			return stored, nil
		}
	}
	return extension, nil
}

func (m *Manager) EnabledExtensionDirs() []string {
	if m == nil || m.ExtensionDAO == nil {
		return nil
	}
	items, err := m.ExtensionDAO.ListEnabled()
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(items))
	for _, item := range items {
		dir := strings.TrimSpace(item.InstallDir)
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func (m *Manager) EnabledExtensionDirsForProfile(profileID string) []string {
	dirs, _ := m.enabledExtensionDirsForProfile(profileID)
	return dirs
}

func (m *Manager) enabledExtensionDirsForProfile(profileID string) ([]string, error) {
	if m == nil || m.ExtensionDAO == nil {
		return nil, nil
	}
	settings, err := m.ExtensionDAO.GetProfileSettings(profileID)
	if err != nil || !settings.Configured {
		items, listErr := m.ExtensionDAO.ListEnabled()
		if listErr != nil {
			return nil, listErr
		}
		return existingExtensionDirs(items), nil
	}
	items, err := m.ExtensionDAO.ListByIDs(settings.ExtensionIDs)
	if err != nil {
		return nil, err
	}
	return existingExtensionDirs(items), nil
}

func existingExtensionDirs(items []Extension) []string {
	dirs := make([]string, 0, len(items))
	for _, item := range items {
		dir := strings.TrimSpace(item.InstallDir)
		if dir != "" {
			if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

// PrepareExtensionDirsForProfile creates an immutable package location inside
// the browser profile. Chromium keys extension state to both the extension ID
// and its unpacked path, so using a stable per-profile path is required for
// Local Extension Settings / IndexedDB data to survive application restarts.
// Existing packages are intentionally never replaced during normal launch.
func (m *Manager) PrepareExtensionDirsForProfile(profileID string, userDataDir string) ([]string, error) {
	sourceDirs, err := m.enabledExtensionDirsForProfile(profileID)
	if err != nil {
		return nil, fmt.Errorf("读取实例插件配置失败: %w", err)
	}
	if len(sourceDirs) == 0 {
		return nil, nil
	}
	root := filepath.Join(filepath.Clean(userDataDir), profileExtensionsDir)
	prepared := make([]string, 0, len(sourceDirs))
	for _, sourceDir := range sourceDirs {
		extensionID := NormalizeExtensionID(filepath.Base(filepath.Clean(sourceDir)))
		if extensionID == "" {
			return nil, fmt.Errorf("插件目录缺少有效 ID: %s", sourceDir)
		}
		targetDir := filepath.Join(root, extensionID)
		if _, statErr := os.Stat(filepath.Join(targetDir, "manifest.json")); statErr == nil {
			prepared = append(prepared, targetDir)
			continue
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("检查实例插件目录失败: %w", statErr)
		}
		if err := copyExtensionDirectory(sourceDir, targetDir); err != nil {
			return nil, fmt.Errorf("准备实例插件 %s 失败: %w", extensionID, err)
		}
		prepared = append(prepared, targetDir)
	}
	return prepared, nil
}
