package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetProxyConfigById 根据代理 ID 获取代理配置
func (m *Manager) GetProxyConfigById(proxyId string) (string, bool) {
	if proxy, ok := m.GetProxyByID(proxyId); ok {
		return strings.TrimSpace(proxy.ProxyConfig), true
	}
	return "", false
}

func (m *Manager) managedUserDataRoot() (string, error) {
	if m == nil || m.Config == nil {
		return "", fmt.Errorf("浏览器配置未初始化")
	}
	root := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	root = m.ResolveRelativePath(root)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析用户数据根目录失败: %w", err)
	}
	return filepath.Clean(rootAbs), nil
}

func (m *Manager) normalizeManagedUserDataDir(userDataDir string, profileID string) (string, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		userDataDir = strings.TrimSpace(profileID)
	}
	if userDataDir == "" {
		return "", fmt.Errorf("实例数据目录不能为空")
	}
	if filepath.IsAbs(userDataDir) || filepath.VolumeName(userDataDir) != "" {
		return "", fmt.Errorf("实例数据目录必须位于 Ant Browser 管理的数据根目录内")
	}
	clean := filepath.Clean(userDataDir)
	if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("实例数据目录不能越过用户数据根目录")
	}

	rootAbs, err := m.managedUserDataRoot()
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, clean))
	if err != nil {
		return "", fmt.Errorf("解析实例数据目录失败: %w", err)
	}
	targetAbs = filepath.Clean(targetAbs)
	if samePath(targetAbs, rootAbs) || !isPathInside(targetAbs, rootAbs) {
		return "", fmt.Errorf("实例数据目录不能越过用户数据根目录")
	}

	if resolvedTarget, err := filepath.EvalSymlinks(targetAbs); err == nil {
		resolvedRoot := rootAbs
		if candidateRoot, rootErr := filepath.EvalSymlinks(rootAbs); rootErr == nil {
			resolvedRoot = candidateRoot
		}
		resolvedTarget = filepath.Clean(resolvedTarget)
		resolvedRoot = filepath.Clean(resolvedRoot)
		if samePath(resolvedTarget, resolvedRoot) || !isPathInside(resolvedTarget, resolvedRoot) {
			return "", fmt.Errorf("实例数据目录通过符号链接越过用户数据根目录")
		}
	}
	return clean, nil
}

// ResolveUserDataDirChecked resolves a managed profile directory and rejects
// absolute, traversal and symlink escape paths.
func (m *Manager) ResolveUserDataDirChecked(profile *Profile) (string, error) {
	if profile == nil {
		return "", fmt.Errorf("profile is nil")
	}
	relative, err := m.normalizeManagedUserDataDir(profile.UserDataDir, profile.ProfileId)
	if err != nil {
		return "", err
	}
	root, err := m.managedUserDataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, relative), nil
}

// ResolveUserDataDir is kept for legacy callers. Invalid persisted paths are
// contained by falling back to the profile's own managed directory.
func (m *Manager) ResolveUserDataDir(profile *Profile) string {
	if resolved, err := m.ResolveUserDataDirChecked(profile); err == nil {
		return resolved
	}
	root, err := m.managedUserDataRoot()
	if err != nil {
		return ""
	}
	profileID := "unknown"
	if profile != nil {
		profileID = safeProfilePathSegment(profile.ProfileId)
	}
	return filepath.Join(root, profileID)
}

// MigrateConfig 迁移旧配置到新格式
func (m *Manager) MigrateConfig() bool {
	log := logger.New("Browser")

	// 如果存在 environments 但没有 cores，执行迁移
	if len(m.Config.Browser.Environments) > 0 && len(m.Config.Browser.Cores) == 0 {
		log.Info("检测到旧配置格式，开始迁移")
		previousBrowser := m.Config.Browser

		for _, env := range m.Config.Browser.Environments {
			m.Config.Browser.Cores = append(m.Config.Browser.Cores, Core{
				CoreId:    env.CoreId,
				CoreName:  env.CoreName,
				CorePath:  env.CorePath,
				IsDefault: env.IsDefault,
			})
		}

		// 清空旧字段
		m.Config.Browser.Environments = nil
		m.Config.Browser.ChromeBinaryPath = ""
		m.Config.Browser.CoreRoot = ""
		m.Config.Browser.DefaultCoreId = ""
		m.Config.Browser.DefaultConnectorType = ""

		if err := m.Config.Save(m.ResolveRelativePath("config.yaml")); err != nil {
			m.Config.Browser = previousBrowser
			log.Error("配置迁移保存失败", logger.F("error", err.Error()))
			return false
		}

		log.Info("配置迁移完成", logger.F("cores_count", len(m.Config.Browser.Cores)))
		return true
	}

	return false
}
