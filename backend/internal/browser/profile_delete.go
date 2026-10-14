package browser

import (
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/profiletxn"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const profileTrashRetention = 72 * time.Hour

type profileDeletionStagedPath struct {
	original string
	staged   string
}

type profileDeletionTransaction struct {
	fileTransaction          *profiletxn.Transaction
	sqliteDAO                *SQLiteProfileDAO
	manager                  *Manager
	profile                  *Profile
	launchCode               string
	launchCodeRemoved        bool
	extensionSettings        ProfileExtensionSettings
	extensionSettingsExisted bool
	extensionRuntimes        []ProfileExtensionRuntime
	extensionSettingsRemoved bool
	extensionRuntimesRemoved bool
	stagedPaths              []profileDeletionStagedPath
}

// Delete 将配置移入回收站
func (m *Manager) Delete(profileId string) error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		log.Error("浏览器配置不存在", logger.F("profile_id", profileId))
		return fmt.Errorf("profile not found")
	}
	if ProfileRuntimeMutationBlocked(profile) || profile.Running || profile.DebugReady || profile.Pid > 0 {
		log.Warn("拒绝删除运行中的浏览器配置", logger.F("profile_id", profileId), logger.F("pid", profile.Pid), logger.F("debug_port", profile.DebugPort))
		return fmt.Errorf("profile runtime is %s; stop it before deletion", NormalizeProfileRuntimeState(profile))
	}
	deletedAt := time.Now().Format(time.RFC3339)
	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.SoftDelete(profileId, deletedAt); err != nil {
			log.Error("数据库移入回收站失败", logger.F("profile_id", profileId), logger.F("error", err))
			return err
		}
	} else {
		previousDeletedAt := profile.DeletedAt
		previousUpdatedAt := profile.UpdatedAt
		profile.DeletedAt = deletedAt
		profile.UpdatedAt = deletedAt
		delete(m.Profiles, profileId)
		if err := m.SaveProfiles(); err != nil {
			profile.DeletedAt = previousDeletedAt
			profile.UpdatedAt = previousUpdatedAt
			m.Profiles[profileId] = profile
			return err
		}
	}
	profile.DeletedAt = deletedAt
	profile.UpdatedAt = deletedAt
	delete(m.Profiles, profileId)
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExists := pathExists(resolvedDir)
	if err := m.deleteProfileFingerprintCheckDir(profile.ProfileId); err != nil {
		log.Error("删除实例指纹检测页缓存失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
	}
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "soft_delete",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            deletedAt,
		DataDirExistedBefore: dataDirExists,
		DataDirExistsAfter:   dataDirExists,
		Success:              true,
	})
	log.Info("浏览器配置移入回收站", logger.F("profile_id", profileId))

	return nil
}

// ListDeleted 获取回收站实例
func (m *Manager) ListDeleted() []Profile {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.cleanupExpiredTrashLocked(log)
	if m.ProfileDAO == nil {
		return []Profile{}
	}
	profiles, err := m.ProfileDAO.ListDeleted()
	if err != nil {
		log.Error("查询回收站实例失败", logger.F("error", err))
		return []Profile{}
	}
	list := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		p := *profile
		if m.CodeProvider != nil {
			if code, err := m.CodeProvider.EnsureCode(p.ProfileId); err == nil {
				p.LaunchCode = code
			}
		}
		list = append(list, p)
	}
	return list
}

// Restore 从回收站恢复实例
func (m *Manager) Restore(profileId string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.ProfileDAO == nil {
		return nil, fmt.Errorf("当前环境不支持回收站恢复")
	}
	profile, err := m.ProfileDAO.GetById(profileId)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.DeletedAt) == "" {
		return nil, fmt.Errorf("实例不在回收站")
	}
	if err := m.ProfileDAO.Restore(profileId); err != nil {
		return nil, err
	}
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExists := pathExists(resolvedDir)
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "restore",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            profile.DeletedAt,
		DataDirExistedBefore: dataDirExists,
		DataDirExistsAfter:   dataDirExists,
		Success:              true,
	})
	profile.DeletedAt = ""
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	profile.CoreId = normalizeProfileCoreID(profile.CoreId)
	profile.RuntimeState = RuntimeStopped
	profile.Running = false
	profile.DebugReady = false
	profile.DebugPort = 0
	profile.Pid = 0
	m.Profiles[profile.ProfileId] = profile
	log.Info("实例已从回收站恢复", logger.F("profile_id", profileId))
	return profile, nil
}

// PermanentlyDelete 从回收站彻底删除实例及其关联数据
func (m *Manager) PermanentlyDelete(profileId string) error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if m.ProfileDAO == nil {
		return fmt.Errorf("当前环境不支持回收站物理删除")
	}
	profile, err := m.ProfileDAO.GetById(profileId)
	if err != nil {
		return err
	}
	if strings.TrimSpace(profile.DeletedAt) == "" {
		return fmt.Errorf("只能彻底删除回收站内的实例")
	}
	resolvedDir := m.ResolveUserDataDir(profile)
	dataDirExistedBefore := pathExists(resolvedDir)
	deleteTx, err := m.prepareProfileDeletionTransactionLocked(log, profile)
	if err != nil {
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "permanent_delete",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              false,
			Error:                err.Error(),
		})
		return err
	}
	if err := deleteTx.deleteRecord(); err != nil {
		if errors.Is(err, profiletxn.ErrCommitUncertain) {
			return err
		}
		rollbackErr := deleteTx.rollback()
		combinedErr := errors.Join(err, rollbackErr)
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "permanent_delete",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              false,
			Error:                combinedErr.Error(),
		})
		return combinedErr
	}
	deleteTx.commit(log)
	m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
		Action:               "permanent_delete",
		ProfileID:            profile.ProfileId,
		ProfileName:          profile.ProfileName,
		UserDataDir:          profile.UserDataDir,
		ResolvedDir:          resolvedDir,
		DeletedAt:            profile.DeletedAt,
		DataDirExistedBefore: dataDirExistedBefore,
		DataDirExistsAfter:   pathExists(resolvedDir),
		Success:              true,
	})
	log.Info("回收站实例已彻底删除", logger.F("profile_id", profileId))
	return nil
}

// CleanupExpiredTrash 清理超过保留期的回收站实例
func (m *Manager) CleanupExpiredTrash() error {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	return m.cleanupExpiredTrashLocked(log)
}

func (m *Manager) cleanupExpiredTrashLocked(log *logger.Logger) error {
	if m.ProfileDAO == nil {
		return nil
	}
	expiredBefore := time.Now().Add(-profileTrashRetention).Format(time.RFC3339)
	expired, err := m.ProfileDAO.ListExpiredDeleted(expiredBefore)
	if err != nil {
		log.Error("清理过期回收站实例失败", logger.F("error", err))
		return err
	}
	cleaned := 0
	cleanupErrors := make([]error, 0)
	for _, profile := range expired {
		resolvedDir := m.ResolveUserDataDir(profile)
		dataDirExistedBefore := pathExists(resolvedDir)
		deleteTx, err := m.prepareProfileDeletionTransactionLocked(log, profile)
		if err != nil {
			log.Error("清理过期回收站实例关联数据失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err))
			cleanupErrors = append(cleanupErrors, fmt.Errorf("清理过期实例 %s 关联数据失败: %w", profile.ProfileId, err))
			m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
				Action:               "expired_cleanup",
				ProfileID:            profile.ProfileId,
				ProfileName:          profile.ProfileName,
				UserDataDir:          profile.UserDataDir,
				ResolvedDir:          resolvedDir,
				DeletedAt:            profile.DeletedAt,
				DataDirExistedBefore: dataDirExistedBefore,
				DataDirExistsAfter:   pathExists(resolvedDir),
				Success:              false,
				Error:                err.Error(),
			})
			continue
		}
		if err := deleteTx.deleteRecord(); err != nil {
			if errors.Is(err, profiletxn.ErrCommitUncertain) {
				return errors.Join(append(cleanupErrors, err)...)
			}
			rollbackErr := deleteTx.rollback()
			combinedErr := errors.Join(err, rollbackErr)
			cleanupErrors = append(cleanupErrors, fmt.Errorf("删除过期实例 %s 失败: %w", profile.ProfileId, combinedErr))
			log.Error("删除过期回收站实例记录失败", logger.F("profile_id", profile.ProfileId), logger.F("error", combinedErr))
			m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
				Action:               "expired_cleanup",
				ProfileID:            profile.ProfileId,
				ProfileName:          profile.ProfileName,
				UserDataDir:          profile.UserDataDir,
				ResolvedDir:          resolvedDir,
				DeletedAt:            profile.DeletedAt,
				DataDirExistedBefore: dataDirExistedBefore,
				DataDirExistsAfter:   pathExists(resolvedDir),
				Success:              false,
				Error:                combinedErr.Error(),
			})
			continue
		}
		deleteTx.commit(log)
		m.writeProfileDeleteAudit(log, profileDeleteAuditEntry{
			Action:               "expired_cleanup",
			ProfileID:            profile.ProfileId,
			ProfileName:          profile.ProfileName,
			UserDataDir:          profile.UserDataDir,
			ResolvedDir:          resolvedDir,
			DeletedAt:            profile.DeletedAt,
			DataDirExistedBefore: dataDirExistedBefore,
			DataDirExistsAfter:   pathExists(resolvedDir),
			Success:              true,
		})
		cleaned++
	}
	if cleaned > 0 {
		log.Info("过期回收站实例已清理", logger.F("count", cleaned))
	}
	return errors.Join(cleanupErrors...)
}

func (m *Manager) prepareProfileDeletionTransactionLocked(log *logger.Logger, profile *Profile) (*profileDeletionTransaction, error) {
	if profile == nil {
		return &profileDeletionTransaction{manager: m}, nil
	}
	if err := m.checkProfileDirectoryOwnershipLocked(profile.ProfileId, m.ResolveUserDataDir(profile)); err != nil {
		return nil, err
	}
	tx := &profileDeletionTransaction{manager: m, profile: profile}
	if dao, ok := m.ProfileDAO.(*SQLiteProfileDAO); ok {
		paths, err := m.profileDeletionManagedPaths(profile)
		if err != nil {
			return nil, err
		}
		root, err := m.managedUserDataRoot()
		if err != nil {
			return nil, err
		}
		plan := profiletxn.Plan{Kind: "delete"}
		for _, path := range paths {
			plan.Moves = append(plan.Moves, profiletxn.Move{Target: path})
		}
		tx.sqliteDAO = dao
		tx.fileTransaction, err = profiletxn.Begin(dao.db, plan, []string{root, m.ResolveRelativePath("data")})
		if err != nil {
			return nil, err
		}
		if err := tx.fileTransaction.Apply(); err != nil {
			return nil, errors.Join(err, tx.rollback())
		}
		return tx, nil
	}

	if m.CodeProvider != nil {
		if code, ok := m.CodeProvider.LookupCode(profile.ProfileId); ok {
			tx.launchCode = code
		}
	}
	if m.ExtensionDAO != nil {
		settings, err := m.ExtensionDAO.GetProfileSettings(profile.ProfileId)
		if err != nil {
			return nil, fmt.Errorf("读取实例插件配置失败: %w", err)
		}
		tx.extensionSettings = settings
		tx.extensionSettingsExisted = strings.TrimSpace(settings.UpdatedAt) != ""
		runtimes, err := m.ExtensionDAO.ListProfileExtensionRuntime(profile.ProfileId)
		if err != nil {
			return nil, fmt.Errorf("读取实例插件运行态失败: %w", err)
		}
		tx.extensionRuntimes = append([]ProfileExtensionRuntime(nil), runtimes...)
	}

	staged, err := m.stageProfileDeletionPaths(profile)
	if err != nil {
		return nil, err
	}
	tx.stagedPaths = staged

	rollbackOnError := func(cause error) (*profileDeletionTransaction, error) {
		rollbackErr := tx.rollback()
		return nil, errors.Join(cause, rollbackErr)
	}
	if m.CodeProvider != nil && strings.TrimSpace(tx.launchCode) != "" {
		if err := m.CodeProvider.Remove(profile.ProfileId); err != nil {
			return rollbackOnError(fmt.Errorf("删除实例 LaunchCode 失败: %w", err))
		}
		tx.launchCodeRemoved = true
	}
	if m.ExtensionDAO != nil {
		if err := m.ExtensionDAO.DeleteProfileSettings(profile.ProfileId); err != nil {
			return rollbackOnError(fmt.Errorf("删除实例插件配置失败: %w", err))
		}
		tx.extensionSettingsRemoved = true
		if err := m.ExtensionDAO.DeleteProfileExtensionRuntimeForProfile(profile.ProfileId); err != nil {
			return rollbackOnError(fmt.Errorf("删除实例插件运行态失败: %w", err))
		}
		tx.extensionRuntimesRemoved = true
	}
	return tx, nil
}

func (m *Manager) stageProfileDeletionPaths(profile *Profile) ([]profileDeletionStagedPath, error) {
	paths, err := m.profileDeletionManagedPaths(profile)
	if err != nil {
		return nil, err
	}
	staged := make([]profileDeletionStagedPath, 0, len(paths))
	for index, target := range paths {
		if _, err := os.Lstat(target); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		stagingPath := filepath.Join(filepath.Dir(target), fmt.Sprintf(".%s.delete-staging-%d-%d", filepath.Base(target), time.Now().UnixNano(), index))
		if err := os.Rename(target, stagingPath); err != nil {
			rollbackErrors := make([]error, 0)
			for i := len(staged) - 1; i >= 0; i-- {
				if restoreErr := os.Rename(staged[i].staged, staged[i].original); restoreErr != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复已暂存实例目录失败 %s: %w", staged[i].original, restoreErr))
				}
			}
			return nil, errors.Join(fmt.Errorf("暂存实例关联目录失败 %s: %w", target, err), errors.Join(rollbackErrors...))
		}
		staged = append(staged, profileDeletionStagedPath{original: target, staged: stagingPath})
	}
	return staged, nil
}

func (m *Manager) profileDeletionManagedPaths(profile *Profile) ([]string, error) {
	if profile == nil {
		return nil, nil
	}
	paths := make([]string, 0, 3)
	userDataRoot := strings.TrimSpace(m.Config.Browser.UserDataRoot)
	if userDataRoot == "" {
		userDataRoot = "data"
	}
	rootAbs, err := filepath.Abs(m.ResolveRelativePath(userDataRoot))
	if err != nil {
		return nil, err
	}
	userDataDir, err := filepath.Abs(m.ResolveUserDataDir(profile))
	if err != nil {
		return nil, err
	}
	if !samePath(userDataDir, rootAbs) && isPathInside(userDataDir, rootAbs) {
		paths = append(paths, filepath.Clean(userDataDir))
	}

	dataRoot, err := filepath.Abs(m.ResolveRelativePath("data"))
	if err != nil {
		return nil, err
	}
	snapshotRoot := filepath.Join(dataRoot, "snapshots")
	snapshotPath, err := filepath.Abs(filepath.Join(snapshotRoot, safeProfilePathSegment(profile.ProfileId)))
	if err != nil {
		return nil, err
	}
	if !samePath(snapshotPath, snapshotRoot) && isPathInside(snapshotPath, snapshotRoot) {
		paths = append(paths, filepath.Clean(snapshotPath))
	}
	fingerprintRoot := filepath.Join(dataRoot, "fingerprint-check")
	fingerprintPath, err := filepath.Abs(filepath.Join(fingerprintRoot, safeProfilePathSegment(profile.ProfileId)))
	if err != nil {
		return nil, err
	}
	if !samePath(fingerprintPath, fingerprintRoot) && isPathInside(fingerprintPath, fingerprintRoot) {
		paths = append(paths, filepath.Clean(fingerprintPath))
	}
	return paths, nil
}

func (tx *profileDeletionTransaction) rollback() error {
	if tx == nil || tx.manager == nil || tx.profile == nil {
		return nil
	}
	if tx.fileTransaction != nil {
		_, err := tx.fileTransaction.Resolve()
		return err
	}
	m := tx.manager
	rollbackErrors := make([]error, 0)
	if m.ExtensionDAO != nil {
		if tx.extensionSettingsRemoved {
			if tx.extensionSettingsExisted {
				if _, err := m.ExtensionDAO.SetProfileSettings(tx.profile.ProfileId, tx.extensionSettings.ExtensionIDs, tx.extensionSettings.Configured); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例插件配置失败: %w", err))
				}
			} else if err := m.ExtensionDAO.DeleteProfileSettings(tx.profile.ProfileId); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例空插件配置失败: %w", err))
			}
		}
		if tx.extensionRuntimesRemoved {
			if err := m.ExtensionDAO.DeleteProfileExtensionRuntimeForProfile(tx.profile.ProfileId); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("清理实例插件运行态回滚目标失败: %w", err))
			}
			for _, runtimeState := range tx.extensionRuntimes {
				if err := m.ExtensionDAO.UpsertProfileExtensionRuntime(runtimeState); err != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例插件运行态失败: %w", err))
				}
			}
		}
	}
	if tx.launchCodeRemoved && m.CodeProvider != nil && strings.TrimSpace(tx.launchCode) != "" {
		if _, err := m.CodeProvider.SetCode(tx.profile.ProfileId, tx.launchCode); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例 LaunchCode 失败: %w", err))
		}
	}
	for i := len(tx.stagedPaths) - 1; i >= 0; i-- {
		entry := tx.stagedPaths[i]
		if _, err := os.Lstat(entry.staged); err != nil {
			if !os.IsNotExist(err) {
				rollbackErrors = append(rollbackErrors, err)
			}
			continue
		}
		if _, err := os.Lstat(entry.original); err == nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例目录目标已存在: %s", entry.original))
			continue
		} else if !os.IsNotExist(err) {
			rollbackErrors = append(rollbackErrors, err)
			continue
		}
		if err := os.Rename(entry.staged, entry.original); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复实例目录失败 %s: %w", entry.original, err))
		}
	}
	return errors.Join(rollbackErrors...)
}

func (tx *profileDeletionTransaction) commit(log *logger.Logger) {
	if tx == nil {
		return
	}
	if tx.fileTransaction != nil {
		if provider, ok := tx.manager.CodeProvider.(interface{ ForgetCode(string) }); ok {
			provider.ForgetCode(tx.profile.ProfileId)
		}
		if _, err := tx.fileTransaction.Resolve(); err != nil && log != nil {
			log.Error("实例删除已提交，暂存清理将在重启时重试", logger.F("error", err))
		}
		return
	}
	for _, entry := range tx.stagedPaths {
		if err := os.RemoveAll(entry.staged); err != nil && log != nil {
			log.Error("清理实例删除暂存目录失败", logger.F("dir", entry.staged), logger.F("error", err))
		}
	}
}

func (tx *profileDeletionTransaction) deleteRecord() error {
	if tx.fileTransaction == nil {
		return tx.manager.ProfileDAO.Delete(tx.profile.ProfileId)
	}
	dbtx, err := tx.sqliteDAO.db.Begin()
	if err != nil {
		return err
	}
	defer dbtx.Rollback()
	for _, table := range []string{"browser_profile_extension_runtime", "browser_profile_extensions", "browser_profile_extension_settings", "launch_codes", "browser_profiles"} {
		if _, err := dbtx.Exec("DELETE FROM "+table+" WHERE profile_id = ?", tx.profile.ProfileId); err != nil {
			return err
		}
	}
	return tx.fileTransaction.Commit(dbtx)
}

func (m *Manager) deleteProfileFingerprintCheckDir(profileId string) error {
	profileId = strings.TrimSpace(profileId)
	if profileId == "" {
		return nil
	}
	dataRoot, err := filepath.Abs(m.ResolveRelativePath("data"))
	if err != nil {
		return fmt.Errorf("解析数据根目录失败: %w", err)
	}
	fingerprintRoot := filepath.Join(dataRoot, "fingerprint-check")
	target, err := filepath.Abs(filepath.Join(fingerprintRoot, safeProfilePathSegment(profileId)))
	if err != nil {
		return fmt.Errorf("解析指纹检测页缓存目录失败: %w", err)
	}
	dataRoot = filepath.Clean(dataRoot)
	fingerprintRoot = filepath.Clean(fingerprintRoot)
	target = filepath.Clean(target)
	if samePath(target, fingerprintRoot) || samePath(target, dataRoot) || !isPathInside(target, fingerprintRoot) {
		return nil
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("删除指纹检测页缓存目录失败: %w", err)
	}
	return nil
}

func safeProfilePathSegment(value string) string {
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

func samePath(a string, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isPathInside(path string, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil || rel == "." || rel == "" {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
