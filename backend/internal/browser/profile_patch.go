package browser

import (
	"fmt"
	"strings"
	"time"

	"ant-chrome/backend/internal/logger"
)

// ProfilePatchInput changes only fields explicitly supplied by the caller.
// A nil pointer means keep the existing value; an empty slice clears a list.
type ProfilePatchInput struct {
	ProfileName   *string   `json:"profileName,omitempty"`
	UserDataDir   *string   `json:"userDataDir,omitempty"`
	CoreId        *string   `json:"coreId,omitempty"`
	ProxyId       *string   `json:"proxyId,omitempty"`
	GroupId       *string   `json:"groupId,omitempty"`
	Tags          *[]string `json:"tags,omitempty"`
	Keywords      *[]string `json:"keywords,omitempty"`
	MemoryLimitMB *int      `json:"memoryLimitMb,omitempty"`
}

func (p ProfilePatchInput) Empty() bool {
	return p.ProfileName == nil && p.UserDataDir == nil && p.CoreId == nil &&
		p.ProxyId == nil && p.GroupId == nil && p.Tags == nil &&
		p.Keywords == nil && p.MemoryLimitMB == nil
}

// Patch applies a sparse update while holding the profile manager lock.
// Unlike Update, this path must not re-normalize unrelated configuration.
func (m *Manager) Patch(profileId string, input ProfilePatchInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists || profile == nil {
		return nil, fmt.Errorf("profile not found")
	}
	if ProfileRuntimeMutationBlocked(profile) {
		return nil, fmt.Errorf("profile runtime is %s; stop it before changing runtime-sensitive settings", NormalizeProfileRuntimeState(profile))
	}
	if input.Empty() {
		snapshot := cloneProfileValue(profile)
		return &snapshot, nil
	}

	next := cloneProfileValue(profile)
	if input.ProfileName != nil {
		name := strings.TrimSpace(*input.ProfileName)
		if name == "" {
			return nil, fmt.Errorf("profile name is required")
		}
		next.ProfileName = name
	}
	if input.UserDataDir != nil {
		dir, err := m.normalizeManagedUserDataDir(strings.TrimSpace(*input.UserDataDir), profileId)
		if err != nil {
			return nil, err
		}
		if err := m.checkProfileDirectoryOwnershipLocked(profileId, m.ResolveUserDataDir(&Profile{ProfileId: profileId, UserDataDir: dir})); err != nil {
			return nil, err
		}
		next.UserDataDir = dir
	}
	if input.CoreId != nil {
		next.CoreId = normalizeProfileCoreID(strings.TrimSpace(*input.CoreId))
	}
	if input.ProxyId != nil {
		requestedID := strings.TrimSpace(*input.ProxyId)
		resolved, err := m.resolveProfileProxyInput(requestedID, "")
		if err != nil {
			return nil, err
		}
		if resolved.HasSelectedProxy {
			_ = BindProfileToProxy(&next, resolved.SelectedProxy, true)
		} else if resolved.FallbackToDirect {
			_ = m.bindProfileToDirectProxy(&next)
		} else {
			next.ProxyId = resolved.ProxyId
			next.ProxyConfig = resolved.ProxyConfig
			_ = ClearProfileProxyBinding(&next)
		}
	}
	if input.GroupId != nil {
		next.GroupId = buildProfileGroupID(strings.TrimSpace(*input.GroupId))
	}
	if input.Tags != nil {
		next.Tags = append([]string{}, (*input.Tags)...)
	}
	if input.Keywords != nil {
		next.Keywords = append([]string{}, (*input.Keywords)...)
	}
	if input.MemoryLimitMB != nil {
		next.MemoryLimitMB = normalizeMemoryLimitMB(*input.MemoryLimitMB)
	}

	previous := cloneProfileValue(profile)
	next.UpdatedAt = time.Now().Format(time.RFC3339)
	*profile = next
	if err := m.SaveProfiles(); err != nil {
		*profile = previous
		return nil, err
	}
	log.Info("Profile configuration patched", logger.F("profile_id", profileId))
	return profile, nil
}
