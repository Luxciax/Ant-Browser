package browser

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

const directProxyID = "__direct__"

// ApplyDefaults 应用默认配置
func (m *Manager) ApplyDefaults(profile *Profile) bool {
	log := logger.New("Browser")
	changed := false
	if profile.FingerprintArgs == nil || len(profile.FingerprintArgs) == 0 {
		profile.FingerprintArgs = append([]string{}, m.Config.Browser.DefaultFingerprintArgs...)
		changed = true
	}
	if profile.LaunchArgs == nil || len(profile.LaunchArgs) == 0 {
		profile.LaunchArgs = append([]string{}, m.Config.Browser.DefaultLaunchArgs...)
		changed = true
	}
	if strings.TrimSpace(profile.UserDataDir) == "" {
		profile.UserDataDir = profile.ProfileId
		changed = true
	}
	normalizedCoreID := normalizeProfileCoreID(profile.CoreId)
	if profile.CoreId != normalizedCoreID {
		profile.CoreId = normalizedCoreID
		changed = true
	}
	if profile.CoreId == "" {
		if defaultCore, ok := m.GetDefaultCore(); ok {
			profile.CoreId = defaultCore.CoreId
			changed = true
		}
	}

	bindChanged, boundInPool, bindMode := m.ResolveProfileProxyBinding(profile)
	if bindChanged {
		changed = true
	}
	if bindMode != "" && bindMode != "proxy_id" {
		log.Info("实例代理自动重关联",
			logger.F("profile_id", profile.ProfileId),
			logger.F("proxy_id", profile.ProxyId),
			logger.F("mode", bindMode),
		)
	}

	if strings.TrimSpace(profile.ProxyId) == "" {
		if proxy, ok := m.resolvePoolProxyByConfig(profile.ProxyConfig); ok {
			if BindProfileToProxy(profile, proxy, true) {
				changed = true
			}
			boundInPool = true
		} else if strings.TrimSpace(profile.ProxyConfig) == "" && m.bindProfileToDirectProxy(profile) {
			changed = true
			boundInPool = true
		}
	}

	if profile.ProxyId != "" && !boundInPool {
		missingProxyID := profile.ProxyId
		if strings.TrimSpace(profile.ProxyConfig) != "" {
			profile.ProxyId = ""
			changed = true
			if ClearProfileProxyBinding(profile) {
				changed = true
			}
			log.Warn("实例代理ID未找到，已改为使用实例代理配置",
				logger.F("profile_id", profile.ProfileId),
				logger.F("missing_proxy_id", missingProxyID),
			)
		} else if m.bindProfileToDirectProxy(profile) {
			changed = true
			log.Warn("实例代理未找到，已回退到直连",
				logger.F("profile_id", profile.ProfileId),
				logger.F("missing_proxy_id", missingProxyID),
			)
		}
	}

	return changed
}

func (m *Manager) bindProfileToDirectProxy(profile *Profile) bool {
	if profile == nil {
		return false
	}
	if proxy, ok := m.GetProxyByID(directProxyID); ok {
		return BindProfileToProxy(profile, proxy, true)
	}

	changed := false
	if strings.TrimSpace(profile.ProxyId) != "" {
		profile.ProxyId = ""
		changed = true
	}
	if strings.TrimSpace(profile.ProxyConfig) != "" {
		profile.ProxyConfig = ""
		changed = true
	}
	if ClearProfileProxyBinding(profile) {
		changed = true
	}
	return changed
}

func (m *Manager) resolvePoolProxyByConfig(proxyConfig string) (Proxy, bool) {
	target := normalizeProxyBindValue(proxyConfig)
	if target == "" {
		return Proxy{}, false
	}
	proxies := m.listProxyCatalog()
	return uniqueProxyMatch(proxies, func(item Proxy) bool {
		return normalizeProxyBindValue(item.ProxyConfig) == target
	})
}

// copyKeywords 深拷贝 keywords map
func copyKeywords(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
