package backend

import (
	"ant-chrome/backend/internal/snapshot"
	"strings"
)

// snapshotDir 返回指定实例的快照目录路径（存放在 data/snapshots 下）
func (a *App) snapshotDir(profileId string) (string, error) {
	return snapshot.EnsureDir(a.resolveAppPath("data"), snapshotProfilePathSegment(profileId))
}

func snapshotProfilePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}
