package backend

import (
	"ant-chrome/backend/internal/backup"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const fullLoginArchivePath = "payload/login-state.json"

type fullLoginRecord struct {
	ProfileID  string                       `json:"profileId"`
	ArchiveDir string                       `json:"archiveDir"`
	Envelope   profilePortableLoginEnvelope `json:"envelope"`
}
type fullLoginBundle struct {
	Version  int               `json:"version"`
	Profiles []fullLoginRecord `json:"profiles"`
}

// Add an integrity-covered encrypted component to the existing full backup.
// It works identically for local, OpenList and S3 destinations.
func (a *App) backupAddLoginState(scope *backup.Scope, password string) (int, []string, func(), error) {
	cleanup := func() {}
	bundle := fullLoginBundle{Version: 1}
	warnings := []string{}
	for _, profile := range a.browserMgr.List() {
		dir := a.browserMgr.ResolveUserDataDir(&profile)
		if _, err := os.Stat(filepath.Join(dir, localStateFileName)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return 0, nil, cleanup, err
		}
		archiveDir := ""
		for _, entry := range scope.Entries {
			if entry.EntryType != backup.EntryTypeDir {
				continue
			}
			rel, err := filepath.Rel(entry.SourcePath, dir)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				archiveDir = filepath.ToSlash(filepath.Join(entry.ArchivePath, rel))
				break
			}
		}
		if archiveDir == "" {
			return 0, nil, cleanup, fmt.Errorf("实例「%s」目录不在全量备份范围内，请改用导出实例", profile.ProfileName)
		}
		envelope, err := createProfilePortableLoginEnvelope(dir, password)
		if err == errPortableLoginKeyMissing {
			warnings = append(warnings, fmt.Sprintf("实例「%s」没有可迁移登录密钥或快照", profile.ProfileName))
			continue
		}
		if err != nil {
			return 0, nil, cleanup, fmt.Errorf("实例「%s」登录态迁移准备失败：%w", profile.ProfileName, err)
		}
		bundle.Profiles = append(bundle.Profiles, fullLoginRecord{profile.ProfileId, archiveDir, *envelope})
		if state, stateErr := readLocalLoginState(dir, loginSnapshotFile); stateErr == nil {
			warnings = append(warnings, fmt.Sprintf("实例「%s」快照 %s，%d 条 Cookie；网站可能仍要求验证", profile.ProfileName, state.CapturedAt.Local().Format("2006-01-02 15:04:05"), len(state.Cookies)))
		}
	}
	if len(bundle.Profiles) == 0 {
		return 0, warnings, cleanup, nil
	}
	dir, err := os.MkdirTemp("", "ant-login-export-*")
	if err != nil {
		return 0, nil, cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	data, err := json.Marshal(bundle)
	if err != nil {
		return 0, nil, cleanup, err
	}
	file := filepath.Join(dir, "login-state.json")
	if err = os.WriteFile(file, data, 0600); err != nil {
		return 0, nil, cleanup, err
	}
	scope.Entries = append(scope.Entries, backup.ScopeEntry{ID: "portable_login", Category: backup.CategoryBrowserData, EntryType: backup.EntryTypeFile, Required: true, Exists: true, SourcePath: file, ArchivePath: fullLoginArchivePath, Description: "密码加密的可迁移登录态"})
	return len(bundle.Profiles), warnings, cleanup, nil
}

// Decrypt into extracted staging files BEFORE stopping processes or mutating
// live data. Wrong passwords and invalid records cannot partially import.
func prepareFullBackupLogin(extractRoot, password string) (int, error) {
	file, err := os.Open(filepath.Join(extractRoot, filepath.FromSlash(fullLoginArchivePath)))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (64<<20)+1))
	if err != nil {
		return 0, err
	}
	if len(data) > 64<<20 {
		return 0, fmt.Errorf("全量备份登录态组件过大")
	}
	var bundle fullLoginBundle
	if json.Unmarshal(data, &bundle) != nil || bundle.Version != 1 || len(bundle.Profiles) > 100000 {
		return 0, fmt.Errorf("全量备份登录态组件格式无效")
	}
	if strings.TrimSpace(password) == "" {
		return len(bundle.Profiles), nil
	}
	seen := map[string]bool{}
	for _, record := range bundle.Profiles {
		archiveDir, err := backupNormalizeManifestPath(record.ArchiveDir)
		if err != nil || !strings.HasPrefix(archiveDir, "payload/") || seen[archiveDir] {
			return 0, fmt.Errorf("登录态恢复目录无效或重复")
		}
		seen[archiveDir] = true
		dir := filepath.Join(extractRoot, filepath.FromSlash(archiveDir))
		if _, err := os.Stat(filepath.Join(dir, localStateFileName)); err != nil {
			return 0, fmt.Errorf("登录态对应的浏览器数据缺失，已中止导入")
		}
		if err := restoreProfilePortableLoginEnvelope(dir, password, record.Envelope); err != nil {
			return 0, err
		}
	}
	return len(bundle.Profiles), nil
}

func prepareFullBackupSameDeviceLogin(extractRoot string) []string {
	var warnings []string
	for _, root := range []string{"payload/app/data", "payload/browser/user-data"} {
		err := filepath.WalkDir(filepath.Join(extractRoot, filepath.FromSlash(root)), func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !entry.IsDir() && entry.Name() == loginSnapshotFile {
				if _, err := prepareSameDeviceLogin(filepath.Dir(path)); err != nil {
					warnings = append(warnings, "部分实例的本机登录态快照无法解密；跨设备恢复请使用带迁移密码的备份")
				}
			}
			return nil
		})
		if err != nil {
			warnings = append(warnings, "部分本机登录态快照未能检查")
		}
	}
	return warnings
}

// Called only after the password prompt. Password is never persisted or logged.
func (a *App) BackupRestoreWithLoginPassword(zipPath, password string) (map[string]interface{}, error) {
	if err := a.lockBackupImportMaintenance(); err != nil {
		return nil, err
	}
	defer a.maintenanceMu.Unlock()
	format, err := detectBackupPackageFormat(zipPath)
	if err != nil {
		return nil, err
	}
	if format != backup.PackageFormat {
		return nil, fmt.Errorf("此接口仅支持全量备份")
	}
	if len(strings.TrimSpace(password)) < 8 {
		return nil, fmt.Errorf("请输入导出时的迁移密码（至少 8 个字符）")
	}
	return a.backupImportFromPathWithPasswordLocked(zipPath, password)
}
