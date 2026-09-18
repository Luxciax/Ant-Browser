package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/snapshot"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

var browserSnapshotRestoreUnzipLimits = snapshot.UnzipLimits{
	MaxEntries:           200000,
	MaxUncompressedBytes: 64 * 1024 * 1024 * 1024,
	MaxSingleFileBytes:   16 * 1024 * 1024 * 1024,
	MaxCompressedBytes:   64 * 1024 * 1024 * 1024,
}

// getProfileForSnapshot 获取实例信息（加锁）
func (a *App) getProfileForSnapshot(profileId string) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	profile, exists := a.browserMgr.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("实例不存在: %s", profileId)
	}
	copyProfile := *profile
	copyProfile.FingerprintArgs = append([]string{}, profile.FingerprintArgs...)
	copyProfile.LaunchArgs = append([]string{}, profile.LaunchArgs...)
	copyProfile.LastLaunchArgs = append([]string{}, profile.LastLaunchArgs...)
	copyProfile.Tags = append([]string{}, profile.Tags...)
	copyProfile.Keywords = append([]string{}, profile.Keywords...)
	return &copyProfile, nil
}

// BrowserSnapshotCreate 创建快照
func (a *App) BrowserSnapshotCreate(profileId, name string) (SnapshotInfo, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()

	profile, err := a.getProfileForSnapshot(profileId)
	if err != nil {
		return SnapshotInfo{}, err
	}
	if browser.ProfileRuntimeMutationBlocked(profile) {
		return SnapshotInfo{}, fmt.Errorf("请先停止实例再创建快照")
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if _, err := os.Stat(userDataDir); os.IsNotExist(err) {
		return SnapshotInfo{}, fmt.Errorf("用户数据目录不存在，无法创建快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return SnapshotInfo{}, err
	}

	snapshotID := uuid.NewString()
	safeName := safeSnapshotFileSegment(name)
	fileStem := snapshotID
	if safeName != "" {
		fileStem += "_" + safeName
	}
	zipPath := filepath.Join(snapDir, fileStem+".zip")
	metaPath := filepath.Join(snapDir, fileStem+".meta.json")
	tempFile, err := os.CreateTemp(snapDir, "."+snapshotID+".snapshot-*.zip")
	if err != nil {
		return SnapshotInfo{}, fmt.Errorf("创建快照临时文件失败: %w", err)
	}
	tempZipPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempZipPath)
		return SnapshotInfo{}, fmt.Errorf("关闭快照临时文件失败: %w", err)
	}
	_ = os.Remove(tempZipPath)
	defer os.Remove(tempZipPath)

	if err := snapshot.ZipDir(userDataDir, tempZipPath); err != nil {
		return SnapshotInfo{}, fmt.Errorf("压缩失败: %w", err)
	}
	if err := fsutil.ReplaceFile(tempZipPath, zipPath); err != nil {
		return SnapshotInfo{}, fmt.Errorf("提交快照文件失败: %w", err)
	}

	fi, err := os.Stat(zipPath)
	if err != nil {
		_ = os.Remove(zipPath)
		return SnapshotInfo{}, err
	}
	sizeMB := float64(fi.Size()) / 1024 / 1024

	info := SnapshotInfo{
		SnapshotId: snapshotID,
		ProfileId:  profileId,
		Name:       name,
		SizeMB:     sizeMB,
		CreatedAt:  time.Now().Format(time.RFC3339),
		FilePath:   zipPath,
	}

	metaData, err := json.Marshal(info)
	if err != nil {
		removeErr := os.Remove(zipPath)
		return SnapshotInfo{}, errors.Join(fmt.Errorf("生成快照元数据失败: %w", err), removeErr)
	}
	if err := fsutil.AtomicWriteFile(metaPath, metaData, 0o644); err != nil {
		removeErr := os.Remove(zipPath)
		return SnapshotInfo{}, errors.Join(fmt.Errorf("写入快照元数据失败: %w", err), removeErr)
	}

	info.FilePath = ""
	return info, nil
}

// BrowserSnapshotList 列出实例的所有快照
func (a *App) BrowserSnapshotList(profileId string) ([]SnapshotInfo, error) {
	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(snapDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil
		}
		return nil, err
	}

	var list []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(snapDir, entry.Name()))
		if err != nil {
			continue
		}
		var info SnapshotInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		info.FilePath = ""
		list = append(list, info)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	return list, nil
}

// BrowserSnapshotRestore 恢复快照
func (a *App) BrowserSnapshotRestore(profileId, snapshotId string) error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()

	profile, err := a.getProfileForSnapshot(profileId)
	if err != nil {
		return err
	}
	if browser.ProfileRuntimeMutationBlocked(profile) {
		return fmt.Errorf("请先停止实例再恢复快照")
	}

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}

	_, zipPath, err := snapshot.FindFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	parentDir := filepath.Dir(userDataDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("创建实例数据父目录失败: %w", err)
	}
	stagingDir, err := os.MkdirTemp(parentDir, "."+filepath.Base(userDataDir)+".snapshot-restore-*")
	if err != nil {
		return fmt.Errorf("创建快照恢复暂存目录失败: %w", err)
	}
	stagingCommitted := false
	defer func() {
		if !stagingCommitted {
			_ = os.RemoveAll(stagingDir)
		}
	}()
	if err := snapshot.UnzipToWithLimits(zipPath, stagingDir, browserSnapshotRestoreUnzipLimits); err != nil {
		return fmt.Errorf("解压快照失败: %w", err)
	}

	rollbackDir := filepath.Join(parentDir, "."+filepath.Base(userDataDir)+".snapshot-rollback-"+uuid.NewString())
	hadOriginal := false
	if _, err := os.Lstat(userDataDir); err == nil {
		hadOriginal = true
		if err := os.Rename(userDataDir, rollbackDir); err != nil {
			return fmt.Errorf("暂存当前实例数据失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取当前实例数据目录失败: %w", err)
	}

	if err := os.Rename(stagingDir, userDataDir); err != nil {
		commitErr := fmt.Errorf("提交快照恢复目录失败: %w", err)
		if hadOriginal {
			if restoreErr := os.Rename(rollbackDir, userDataDir); restoreErr != nil {
				return errors.Join(commitErr, fmt.Errorf("恢复原实例数据失败: %w", restoreErr))
			}
		}
		return commitErr
	}
	stagingCommitted = true
	if hadOriginal {
		if err := os.RemoveAll(rollbackDir); err != nil {
			return fmt.Errorf("快照已恢复，但清理旧实例数据暂存失败: %w", err)
		}
	}
	return nil
}

// BrowserSnapshotDelete 删除快照
func (a *App) BrowserSnapshotDelete(profileId, snapshotId string) error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()

	snapDir, err := a.snapshotDir(profileId)
	if err != nil {
		return err
	}
	metaPath, zipPath, err := snapshot.FindFiles(snapDir, snapshotId)
	if err != nil {
		return err
	}
	stageRoot := filepath.Join(snapDir, ".snapshot-delete-"+uuid.NewString())
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return fmt.Errorf("创建快照删除暂存目录失败: %w", err)
	}
	type stagedSnapshotPath struct {
		original string
		staged   string
	}
	staged := make([]stagedSnapshotPath, 0, 2)
	for _, original := range []string{zipPath, metaPath} {
		stagedPath := filepath.Join(stageRoot, filepath.Base(original))
		if err := os.Rename(original, stagedPath); err != nil {
			rollbackErrors := []error{fmt.Errorf("暂存待删除快照文件失败 %s: %w", filepath.Base(original), err)}
			for i := len(staged) - 1; i >= 0; i-- {
				if restoreErr := os.Rename(staged[i].staged, staged[i].original); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复快照文件失败 %s: %w", filepath.Base(staged[i].original), restoreErr))
				}
			}
			_ = os.RemoveAll(stageRoot)
			return errors.Join(rollbackErrors...)
		}
		staged = append(staged, stagedSnapshotPath{original: original, staged: stagedPath})
	}
	if err := os.RemoveAll(stageRoot); err != nil {
		return fmt.Errorf("快照已删除，但清理删除暂存目录失败: %w", err)
	}
	return nil
}

func safeSnapshotFileSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	count := 0
	for _, char := range value {
		if count >= 80 {
			break
		}
		count++
		if char < 0x20 || char == 0x7f || strings.ContainsRune(`<>:"/\|?*`, char) {
			builder.WriteByte('_')
			continue
		}
		builder.WriteRune(char)
	}
	result := strings.Trim(builder.String(), " .")
	if result == "." || result == ".." {
		return ""
	}
	return result
}
