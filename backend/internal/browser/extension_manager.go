package browser

import (
	"ant-chrome/backend/internal/fsutil"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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

func BuildChromeWebStoreSearchURL(query string) string {
	value := strings.TrimSpace(query)
	if value == "" {
		return ""
	}
	return "https://chromewebstore.google.com/search/" + url.PathEscape(value)
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
		result.Installable = false
		result.Message = "已识别插件 ID，但暂时无法读取商店元信息: " + err.Error()
		return result, nil
	}
	zipData, err := normalizeExtensionArchiveData(data)
	if err != nil {
		result.Installable = false
		result.Message = "已识别插件 ID，但插件包格式无法解析: " + err.Error()
		return result, nil
	}
	manifestData, err := readExtensionManifestFromZip(zipData)
	if err != nil {
		result.Installable = false
		result.Message = "已识别插件 ID，但 manifest 无法解析: " + err.Error()
		return result, nil
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		result.Installable = false
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

func (m *Manager) SearchChromeWebStore(query string) ([]ExtensionSearchResult, error) {
	return m.SearchChromeWebStoreWithHTTPClient(context.Background(), query, nil)
}

func (m *Manager) SearchChromeWebStoreWithHTTPClient(ctx context.Context, query string, client *http.Client) ([]ExtensionSearchResult, error) {
	return searchChromeWebStore(ctx, query, client)
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
		resolvedID = extensionIDFromCRX(data)
		if resolvedID == "" {
			resolvedID = extensionIDFromManifest(manifestData)
		}
	}
	if resolvedID == "" {
		return Extension{}, fmt.Errorf("无法识别插件 ID")
	}

	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), resolvedID)
	fileTx, err := beginExtensionInstallFileTransaction(installDir)
	if err != nil {
		return Extension{}, err
	}
	if err := replaceExtensionDirFromZip(zipData, installDir); err != nil {
		return Extension{}, rollbackExtensionInstallFiles(fileTx, err)
	}
	installMode := ExtensionInstallModePersistent
	packagePath := ""
	packageHash := ""
	if isCRXExtensionPackage(data) {
		packageTarget := m.extensionPackagePath(resolvedID, strings.TrimSpace(manifest.Version))
		if err := fileTx.stagePackage(packageTarget); err != nil {
			return Extension{}, rollbackExtensionInstallFiles(fileTx, err)
		}
		packagePath, packageHash, err = m.storeExtensionPackage(resolvedID, strings.TrimSpace(manifest.Version), data)
		if err != nil {
			return Extension{}, rollbackExtensionInstallFiles(fileTx, err)
		}
		installMode = ExtensionInstallModePersistent
	}

	localeMessages := readExtensionLocaleMessagesFromZip(zipData, manifest)
	manifestJSON := string(manifestData)
	extension := Extension{
		ExtensionID:    resolvedID,
		Name:           resolveExtensionMessage(resolveExtensionName(manifest, resolvedID), localeMessages),
		Version:        strings.TrimSpace(manifest.Version),
		Description:    resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:    readExtensionIconDataURLFromZip(zipData, manifest),
		ManifestJSON:   manifestJSON,
		SourceURL:      strings.TrimSpace(sourceURL),
		InstallDir:     installDir,
		InstallMode:    installMode,
		PackagePath:    packagePath,
		PackageHash:    packageHash,
		Enabled:        true,
		DefaultInstall: true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			return Extension{}, rollbackExtensionInstallFiles(fileTx, err)
		}
		fileTx.commit()
		stored, err := m.ExtensionDAO.Get(resolvedID)
		if err == nil {
			return stored, nil
		}
		return extension, nil
	}
	fileTx.commit()
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
	identity, canonicalSource, err := m.resolveLocalExtensionIdentity(normalizedDir)
	if err != nil {
		return Extension{}, err
	}
	manifestPath := filepath.Join(canonicalSource, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, fmt.Errorf("插件目录缺少 manifest.json: %w", err)
	}
	manifest, err := parseExtensionManifest(manifestData)
	if err != nil {
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, err
	}
	managedManifestData, err := manifestWithLocalExtensionKey(manifestData, identity.PublicKey)
	if err != nil {
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, err
	}
	extensionID := identity.ExtensionID
	installDir := filepath.Join(m.ResolveRelativePath(filepath.Join("data", extensionsRootDir)), extensionID)
	fileTx, err := beginExtensionInstallFileTransaction(installDir)
	if err != nil {
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, err
	}
	if err := copyExtensionDirectory(canonicalSource, installDir); err != nil {
		rollbackErr := fileTx.rollback()
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, errors.Join(err, rollbackErr)
	}
	if err := fsutil.AtomicWriteFile(filepath.Join(installDir, "manifest.json"), managedManifestData, 0o644); err != nil {
		rollbackErr := fileTx.rollback()
		if identity.CreatedKey {
			_ = os.Remove(identity.KeyPath)
		}
		return Extension{}, errors.Join(fmt.Errorf("写入本地插件稳定 manifest 失败: %w", err), rollbackErr)
	}
	localeMessages := readExtensionLocaleMessagesFromDir(canonicalSource, manifest)
	extension := Extension{
		ExtensionID:    extensionID,
		Name:           resolveExtensionMessage(resolveExtensionName(manifest, extensionID), localeMessages),
		Version:        strings.TrimSpace(manifest.Version),
		Description:    resolveExtensionDescription(manifest, localeMessages),
		IconDataURL:    readExtensionIconDataURLFromDir(canonicalSource, manifest),
		ManifestJSON:   string(managedManifestData),
		SourceURL:      canonicalSource,
		InstallDir:     installDir,
		InstallMode:    ExtensionInstallModePersistent,
		Enabled:        true,
		DefaultInstall: true,
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.Upsert(extension); err != nil {
			rollbackErr := fileTx.rollback()
			if identity.CreatedKey {
				_ = os.Remove(identity.KeyPath)
			}
			return Extension{}, errors.Join(err, rollbackErr)
		}
		fileTx.commit()
		if stored, err := m.ExtensionDAO.Get(extensionID); err == nil {
			return stored, nil
		}
		return extension, nil
	}
	fileTx.commit()
	return extension, nil
}

func (m *Manager) EnabledExtensionDirs() []string {
	return nil
}

func (m *Manager) EnabledExtensionDirsForProfile(profileID string) []string {
	return nil
}
