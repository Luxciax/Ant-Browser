package browser

import (
	"fmt"
	"path/filepath"
)

// checkProfileDirectoryOwnershipLocked includes trash: those directories still
// contain user data and are eligible for later automatic deletion. Callers hold
// Mutex throughout validation and the configuration/deletion transaction.
func (m *Manager) checkProfileDirectoryOwnershipLocked(profileID, directory string) error {
	profiles := make([]*Profile, 0, len(m.Profiles))
	for _, profile := range m.Profiles {
		profiles = append(profiles, profile)
	}
	if m.ProfileDAO != nil {
		deleted, err := m.ProfileDAO.ListDeleted()
		if err != nil {
			return fmt.Errorf("核对回收站实例目录失败: %w", err)
		}
		profiles = append(profiles, deleted...)
	}
	target := profileOwnershipPath(directory)
	for _, profile := range profiles {
		if profile == nil || profile.ProfileId == profileID {
			continue
		}
		other := profileOwnershipPath(m.ResolveUserDataDir(profile))
		if samePath(target, other) || isPathInside(target, other) || isPathInside(other, target) {
			return fmt.Errorf("实例数据目录与实例 %s（%s）重叠，请使用独立目录", profile.ProfileName, profile.ProfileId)
		}
	}
	return nil
}

func profileOwnershipPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}
