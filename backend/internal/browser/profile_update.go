package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"
)

// Update 更新配置
func (m *Manager) Update(profileId string, input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		log.Error("浏览器配置不存在", logger.F("profile_id", profileId))
		return nil, fmt.Errorf("profile not found")
	}
	if ProfileRuntimeMutationBlocked(profile) {
		return nil, fmt.Errorf("profile runtime is %s; stop it before changing runtime-sensitive settings", NormalizeProfileRuntimeState(profile))
	}
	previous := cloneProfileValue(profile)
	userDataDir, err := m.normalizeManagedUserDataDir(input.UserDataDir, profileId)
	if err != nil {
		return nil, err
	}
	if err := m.checkProfileDirectoryOwnershipLocked(profileId, m.ResolveUserDataDir(&Profile{ProfileId: profileId, UserDataDir: userDataDir})); err != nil {
		return nil, err
	}
	resolvedProxy, err := m.resolveProfileProxyInput(input.ProxyId, input.ProxyConfig)
	if err != nil {
		log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", strings.TrimSpace(input.ProxyId)), logger.F("error", err.Error()))
		return nil, err
	}

	profile.ProfileName = input.ProfileName
	profile.UserDataDir = userDataDir
	profile.CoreId = normalizeProfileCoreID(input.CoreId)
	profile.RestoreLastSession = NormalizeRestoreLastSessionMode(input.RestoreLastSession)
	profile.FingerprintArgs = append([]string{}, input.FingerprintArgs...)
	if resolvedProxy.HasSelectedProxy {
		_ = BindProfileToProxy(profile, resolvedProxy.SelectedProxy, true)
	} else if resolvedProxy.FallbackToDirect {
		_ = m.bindProfileToDirectProxy(profile)
	} else {
		profile.ProxyId = resolvedProxy.ProxyId
		profile.ProxyConfig = resolvedProxy.ProxyConfig
		_ = ClearProfileProxyBinding(profile)
	}
	profile.MemoryLimitMB = normalizeMemoryLimitMB(input.MemoryLimitMB)
	if resolvedProxy.UsedConfigFallback {
		log.Warn("代理ID未命中，已改为使用输入的代理配置",
			logger.F("profile_id", profileId),
			logger.F("proxy_id", strings.TrimSpace(input.ProxyId)),
		)
	}
	profile.LaunchArgs = append([]string{}, input.LaunchArgs...)
	profile.Tags = append([]string{}, input.Tags...)
	profile.Keywords = append([]string{}, input.Keywords...)
	profile.GroupId = buildProfileGroupID(input.GroupId)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("浏览器配置更新", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if err := m.SaveProfiles(); err != nil {
		*profile = previous
		return nil, err
	}
	return profile, nil
}

// SetKeywords 设置实例关键字（独立接口，不影响其他字段）
func (m *Manager) SetKeywords(profileId string, keywords []string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	previousKeywords := append([]string{}, profile.Keywords...)
	previousUpdatedAt := profile.UpdatedAt
	profile.Keywords = append([]string{}, keywords...)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("关键字更新", logger.F("profile_id", profileId))
	if err := m.SaveProfiles(); err != nil {
		profile.Keywords = previousKeywords
		profile.UpdatedAt = previousUpdatedAt
		return nil, err
	}
	return profile, nil
}

func cloneProfileValue(profile *Profile) Profile {
	if profile == nil {
		return Profile{}
	}
	copyProfile := *profile
	copyProfile.FingerprintArgs = append([]string{}, profile.FingerprintArgs...)
	copyProfile.LaunchArgs = append([]string{}, profile.LaunchArgs...)
	copyProfile.LastLaunchArgs = append([]string{}, profile.LastLaunchArgs...)
	copyProfile.Tags = append([]string{}, profile.Tags...)
	copyProfile.Keywords = append([]string{}, profile.Keywords...)
	return copyProfile
}
