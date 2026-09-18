package backend

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"

	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const profilePackageFormat = "ant-chrome-profile-package"

type ProfilePackageManifest struct {
	Format                string   `json:"format"`
	Version               int      `json:"version"`
	ExportedAt            string   `json:"exportedAt"`
	ProfileCount          int      `json:"profileCount"`
	ProfileNames          []string `json:"profileNames,omitempty"`
	DatabaseVersion       int      `json:"databaseVersion,omitempty"`
	PortableLogin         bool     `json:"portableLogin,omitempty"`
	PortableLoginProfiles []string `json:"portableLoginProfiles,omitempty"`
	Extensions            ProfilePackageExtensionManifest `json:"extensions"`
}

type ProfilePackageExtensionManifest struct {
	Portable bool `json:"portable"`
	Count    int  `json:"count"`
}

type ProfilePackageExportOptions struct {
	PortableLogin     bool   `json:"portableLogin"`
	MigrationPassword string `json:"migrationPassword"`
}

type ProfilePackageExportResult struct {
	Cancelled          bool     `json:"cancelled"`
	ZipPath            string   `json:"zipPath"`
	ProfileCount       int      `json:"profileCount"`
	FileCount          int      `json:"fileCount"`
	PortableLoginCount int      `json:"portableLoginCount"`
	Warnings           []string `json:"warnings,omitempty"`
	Message            string   `json:"message"`
}

type ProfilePackageImportResult struct {
	RenamedCount     int               `json:"renamedCount"`
	Cancelled        bool              `json:"cancelled"`
	ImportedCount    int               `json:"importedCount"`
	CreatedCount     int               `json:"createdCount"`
	OverwrittenCount int               `json:"overwrittenCount"`
	ProfileMappings  map[string]string `json:"profileMappings"`
	Warnings         []string          `json:"warnings"`
	Message          string            `json:"message"`
}

type ProfilePackageImportOptions struct {
	Actions           []ProfilePackageImportAction `json:"actions,omitempty"`
	ConflictMode      string                       `json:"conflictMode"`
	ConfirmConflict   bool                         `json:"confirmConflict"`
	MigrationPassword string                       `json:"migrationPassword,omitempty"`
}

type ProfilePackageImportAction struct {
	SourceProfileID string `json:"sourceProfileId"`
	SourceIndex     int    `json:"sourceIndex"`
	Mode            string `json:"mode"`
	ProfileName     string `json:"profileName"`
}

type ProfilePackageImportConflict struct {
	SourceProfileID       string `json:"sourceProfileId"`
	SourceProfileName     string `json:"sourceProfileName"`
	TargetProfileID       string `json:"targetProfileId"`
	TargetProfileName     string `json:"targetProfileName"`
	MatchType             string `json:"matchType"`
	TargetRunning         bool   `json:"targetRunning"`
	TargetDeleted         bool   `json:"targetDeleted"`
	Ambiguous             bool   `json:"ambiguous"`
	TargetMatches         int    `json:"targetMatches"`
	SourceTargetCollision bool   `json:"sourceTargetCollision"`
	SourceNameCollision   bool   `json:"sourceNameCollision"`
}

type ProfilePackageImportPreviewProfile struct {
	SourceIndex           int    `json:"sourceIndex"`
	SourceProfileID       string `json:"sourceProfileId"`
	SourceProfileName     string `json:"sourceProfileName"`
	TargetProfileID       string `json:"targetProfileId"`
	TargetProfileName     string `json:"targetProfileName"`
	MatchType             string `json:"matchType"`
	TargetRunning         bool   `json:"targetRunning"`
	TargetDeleted         bool   `json:"targetDeleted"`
	Ambiguous             bool   `json:"ambiguous"`
	TargetMatches         int    `json:"targetMatches"`
	SourceTargetCollision bool   `json:"sourceTargetCollision"`
	SourceNameCollision   bool   `json:"sourceNameCollision"`
	SuggestedAction       string `json:"suggestedAction"`
	SuggestedProfileName  string `json:"suggestedProfileName"`
	CanOverwrite          bool   `json:"canOverwrite"`
}

type ProfilePackageImportPreview struct {
	Profiles                  []ProfilePackageImportPreviewProfile `json:"profiles"`
	Cancelled                 bool                                 `json:"cancelled"`
	ZipPath                   string                               `json:"zipPath"`
	ProfileCount              int                                  `json:"profileCount"`
	ConflictCount             int                                  `json:"conflictCount"`
	CanOverwrite              bool                                 `json:"canOverwrite"`
	Conflicts                 []ProfilePackageImportConflict       `json:"conflicts"`
	PortableLogin             bool                                 `json:"portableLogin"`
	PortableLoginCount        int                                  `json:"portableLoginCount"`
	RequiresMigrationPassword bool                                 `json:"requiresMigrationPassword"`
	Message                   string                               `json:"message"`
}

const (
	profilePackageImportModeNew       = "new"
	profilePackageImportModeOverwrite = "overwrite"
	profilePackageImportModeRename    = "rename"
	profilePackageImportMatchID       = "profileId"
	profilePackageImportMatchName     = "profileName"
	profilePackageMaxEntries          = 200000
	profilePackageMaxCompressedBytes  = int64(64 * 1024 * 1024 * 1024)
	profilePackageMaxExpandedBytes    = uint64(128 * 1024 * 1024 * 1024)
	profilePackageMaxSingleFileBytes  = uint64(16 * 1024 * 1024 * 1024)
)

type preparedProfilePackageImport struct {
	Profile           browser.Profile
	OldProfileID      string
	ReplacedProfileID string
	FinalDir          string
	StagingDir        string
	HasUserData       bool
	Action            string
	Overwrite         bool
}

type profilePackageContents struct {
	Reader           *zip.ReadCloser
	Manifest         ProfilePackageManifest
	Profiles         []browser.Profile
	DatabaseSnapshot *ProfilePackageDatabase
}

type profilePackageConflictMatch struct {
	Target     *browser.Profile
	MatchType  string
	Ambiguous  bool
	TargetHits []browser.Profile
}

type profilePackageExistingProfiles struct {
	ByID    map[string]browser.Profile
	ByName  map[string][]browser.Profile
	UsedIDs map[string]struct{}
}

type profilePackageDirectorySwap struct {
	FinalDir    string
	BackupDir   string
	HadOriginal bool
}

// BrowserProfilePackageExport 导出选中的实例配置和浏览器用户数据目录。
func (a *App) BrowserProfilePackageExport(profileIds []string) (ProfilePackageExportResult, error) {
	return a.browserProfilePackageExportWithOptions(profileIds, ProfilePackageExportOptions{})
}

func (a *App) BrowserProfilePackageExportWithOptions(profileIds []string, options ProfilePackageExportOptions) (ProfilePackageExportResult, error) {
	return a.browserProfilePackageExportWithOptions(profileIds, options)
}

func (a *App) browserProfilePackageExportWithOptions(profileIds []string, options ProfilePackageExportOptions) (ProfilePackageExportResult, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	if options.PortableLogin && len(strings.TrimSpace(options.MigrationPassword)) < 8 {
		return ProfilePackageExportResult{}, fmt.Errorf("登录态迁移密码至少需要 8 个字符")
	}

	ids := normalizeProfilePackageIDs(profileIds)
	if len(ids) == 0 {
		return ProfilePackageExportResult{}, fmt.Errorf("请选择要导出的实例")
	}
	if a.ctx == nil {
		return ProfilePackageExportResult{}, fmt.Errorf("应用上下文未初始化")
	}

	profiles, err := a.collectProfilesForPackage(ids)
	if err != nil {
		return ProfilePackageExportResult{}, err
	}

	defaultName := backupProfilePackageFileName(profilePackageProfileNames(profiles), time.Now(), false)
	savePath, cancelled, err := a.backupResolveLocalPackagePath(defaultName, "导出实例")
	if err != nil {
		return ProfilePackageExportResult{}, err
	}
	if cancelled || strings.TrimSpace(savePath) == "" {
		return ProfilePackageExportResult{Cancelled: true, Message: "已取消导出"}, nil
	}

	fileCount, portableLoginCount, warnings, err := a.writeProfilePackageWithOptions(savePath, profiles, options)
	if err != nil {
		return ProfilePackageExportResult{}, err
	}
	return ProfilePackageExportResult{
		Cancelled:          false,
		ZipPath:            savePath,
		ProfileCount:       len(profiles),
		FileCount:          fileCount,
		PortableLoginCount: portableLoginCount,
		Warnings:           warnings,
		Message:            "导出完成",
	}, nil
}

// BrowserProfilePackageImport 导入实例包。保留该接口用于兼容旧调用方，默认选择新建。
func (a *App) BrowserProfilePackageImport() (ProfilePackageImportResult, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return ProfilePackageImportResult{}, fmt.Errorf("应用上下文未初始化")
	}
	zipPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入实例",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return ProfilePackageImportResult{}, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(zipPath) == "" {
		return ProfilePackageImportResult{Cancelled: true, Message: "已取消导入"}, nil
	}
	return a.importProfilePackageFromPathWithModeAndConfirmation(zipPath, profilePackageImportModeNew, false)
}

// BrowserProfilePackagePrepareImport 选择实例包并返回冲突预览，不执行导入。
func (a *App) BrowserProfilePackagePrepareImport() (ProfilePackageImportPreview, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	if a.ctx == nil {
		return ProfilePackageImportPreview{}, fmt.Errorf("应用上下文未初始化")
	}
	zipPath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "导入实例",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP 文件 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return ProfilePackageImportPreview{}, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(zipPath) == "" {
		return ProfilePackageImportPreview{Cancelled: true, Message: "已取消导入"}, nil
	}
	return a.prepareProfilePackageImportFromPath(zipPath)
}

// BrowserProfilePackagePrepareImportFromPath 读取指定实例包并返回冲突预览。
func (a *App) BrowserProfilePackagePrepareImportFromPath(zipPath string) (ProfilePackageImportPreview, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.prepareProfilePackageImportFromPath(zipPath)
}

// BrowserProfilePackageImportWithOptions 按用户选择覆盖或新建实例。
func (a *App) BrowserProfilePackageImportWithOptions(zipPath string, options ProfilePackageImportOptions) (ProfilePackageImportResult, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.importProfilePackageFromPathWithModeAndActionsAndPassword(zipPath, options.ConflictMode, options.ConfirmConflict, options.Actions, options.MigrationPassword)
}

func (a *App) collectProfilesForPackage(profileIds []string) ([]browser.Profile, error) {
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profiles := make([]browser.Profile, 0, len(profileIds))
	missing := make([]string, 0)
	running := make([]string, 0)
	for _, id := range profileIds {
		profile := a.browserMgr.Profiles[id]
		if profile == nil {
			missing = append(missing, id)
			continue
		}
		if browser.ProfileRuntimeMutationBlocked(profile) {
			name := strings.TrimSpace(profile.ProfileName)
			if name == "" {
				name = strings.TrimSpace(profile.ProfileId)
			}
			running = append(running, name)
			continue
		}
		copyProfile := *profile
		copyProfile.LaunchCode = ""
		copyProfile.RuntimeState = browser.RuntimeStopped
		copyProfile.Running = false
		copyProfile.DebugPort = 0
		copyProfile.DebugReady = false
		copyProfile.Pid = 0
		copyProfile.RuntimeWarning = ""
		copyProfile.LastError = ""
		profiles = append(profiles, copyProfile)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("实例不存在: %s", strings.Join(missing, ", "))
	}
	if len(running) > 0 {
		return nil, fmt.Errorf("请先停止实例再导出: %s", strings.Join(running, ", "))
	}
	return profiles, nil
}

func (a *App) writeProfilePackage(zipPath string, profiles []browser.Profile) (int, error) {
	databaseSnapshot, err := a.collectProfilePackageDatabase(profiles)
	if err != nil {
		return 0, err
	}
	extensionArtifacts, extensionWarnings := a.collectProfilePackageExtensionArtifacts(&databaseSnapshot)
	databaseSnapshot.Warnings = append(databaseSnapshot.Warnings, extensionWarnings...)
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建导出目录失败: %w", err)
	}
	out, tmpPath, err := createProfilePackageTempFile(zipPath)
	if err != nil {
		return 0, fmt.Errorf("创建导出文件失败: %w", err)
	}
	defer os.Remove(tmpPath)
	zipWriter := zip.NewWriter(out)
	fileCount := 0
	var manifest ProfilePackageManifest

	writeErr := func() error {
		manifest = ProfilePackageManifest{
			Format:          profilePackageFormat,
			Version:         profilePackageVersion,
			ExportedAt:      time.Now().Format(time.RFC3339),
			ProfileCount:    len(profiles),
			ProfileNames:    profilePackageProfileNames(profiles),
			DatabaseVersion: databaseSnapshot.Version,
			Extensions: ProfilePackageExtensionManifest{
				Portable: len(extensionArtifacts) > 0,
				Count:    len(extensionArtifacts),
			},
		}
		if err := writeProfilePackageJSON(zipWriter, "manifest.json", manifest); err != nil {
			return err
		}
		fileCount++
		if err := writeProfilePackageJSON(zipWriter, profilePackageDatabasePath, databaseSnapshot); err != nil {
			return err
		}
		fileCount++
		if err := writeProfilePackageJSON(zipWriter, "profiles.json", profiles); err != nil {
			return err
		}
		fileCount++
		extensionFileCount, err := writeProfilePackageExtensionArtifacts(zipWriter, extensionArtifacts)
		if err != nil {
			return err
		}
		fileCount += extensionFileCount
		for i := range profiles {
			profile := &profiles[i]
			userDataDir := a.browserMgr.ResolveUserDataDir(profile)
			if _, err := os.Stat(userDataDir); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("读取用户数据目录失败: %w", err)
			}
			added, err := writeProfilePackageDir(zipWriter, userDataDir, "user-data/"+profile.ProfileId)
			if err != nil {
				return fmt.Errorf("打包用户数据失败 [%s]: %w", profile.ProfileName, err)
			}
			fileCount += added
		}
		return nil
	}()

	closeZipErr := zipWriter.Close()
	syncFileErr := out.Sync()
	closeFileErr := out.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return 0, writeErr
	}
	if closeZipErr != nil {
		_ = os.Remove(tmpPath)
		return 0, closeZipErr
	}
	if syncFileErr != nil {
		_ = os.Remove(tmpPath)
		return 0, syncFileErr
	}
	if closeFileErr != nil {
		_ = os.Remove(tmpPath)
		return 0, closeFileErr
	}
	if err := fsutil.ReplaceFile(tmpPath, zipPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("保存导出文件失败: %w", err)
	}
	if _, err := backupWriteProfileMetadata(zipPath, manifest, fileCount, a.appName(), a.appVersion()); err != nil {
		logger.New("Backup").Warn("实例包已导出，但元数据写入失败", logger.F("path", zipPath), logger.F("error", err.Error()))
	}
	return fileCount, nil
}

func (a *App) writeProfilePackageWithOptions(zipPath string, profiles []browser.Profile, options ProfilePackageExportOptions) (int, int, []string, error) {
	if !options.PortableLogin {
		count, err := a.writeProfilePackage(zipPath, profiles)
		return count, 0, nil, err
	}

	portableEnvelopes := make(map[string]profilePortableLoginEnvelope, len(profiles))
	portableProfiles := make([]string, 0, len(profiles))
	warnings := make([]string, 0)
	for i := range profiles {
		profile := &profiles[i]
		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		envelope, err := createProfilePortableLoginEnvelope(userDataDir, options.MigrationPassword)
		if err == errPortableLoginKeyMissing {
			warnings = append(warnings, fmt.Sprintf("实例「%s」未找到可迁移的浏览器加密主密钥，登录态按普通实例数据导出", profile.ProfileName))
			continue
		}
		if err != nil {
			return 0, 0, warnings, fmt.Errorf("准备实例登录态迁移数据失败 [%s]: %w", profile.ProfileName, err)
		}
		portableEnvelopes[profile.ProfileId] = *envelope
		portableProfiles = append(portableProfiles, profile.ProfileId)
	}

	databaseSnapshot, err := a.collectProfilePackageDatabase(profiles)
	if err != nil {
		return 0, 0, warnings, err
	}
	extensionArtifacts, extensionWarnings := a.collectProfilePackageExtensionArtifacts(&databaseSnapshot)
	databaseSnapshot.Warnings = append(databaseSnapshot.Warnings, extensionWarnings...)
	warnings = append(warnings, extensionWarnings...)
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return 0, 0, warnings, fmt.Errorf("创建导出目录失败: %w", err)
	}
	out, tmpPath, err := createProfilePackageTempFile(zipPath)
	if err != nil {
		return 0, 0, warnings, fmt.Errorf("创建导出文件失败: %w", err)
	}
	defer os.Remove(tmpPath)
	zipWriter := zip.NewWriter(out)
	fileCount := 0
	manifest := ProfilePackageManifest{
		Format:                profilePackageFormat,
		Version:               profilePackageVersion,
		ExportedAt:            time.Now().Format(time.RFC3339),
		ProfileCount:          len(profiles),
		ProfileNames:          profilePackageProfileNames(profiles),
		DatabaseVersion:       databaseSnapshot.Version,
		PortableLogin:         len(portableProfiles) > 0,
		PortableLoginProfiles: portableProfiles,
		Extensions: ProfilePackageExtensionManifest{
			Portable: len(extensionArtifacts) > 0,
			Count:    len(extensionArtifacts),
		},
	}

	writeErr := func() error {
		if err := writeProfilePackageJSON(zipWriter, "manifest.json", manifest); err != nil {
			return err
		}
		fileCount++
		if err := writeProfilePackageJSON(zipWriter, profilePackageDatabasePath, databaseSnapshot); err != nil {
			return err
		}
		fileCount++
		if err := writeProfilePackageJSON(zipWriter, "profiles.json", profiles); err != nil {
			return err
		}
		fileCount++
		extensionFileCount, err := writeProfilePackageExtensionArtifacts(zipWriter, extensionArtifacts)
		if err != nil {
			return err
		}
		fileCount += extensionFileCount
		for _, profileID := range portableProfiles {
			envelope := portableEnvelopes[profileID]
			if err := writeProfilePackageJSON(zipWriter, profilePortableLoginEntryName(profileID), envelope); err != nil {
				return err
			}
			fileCount++
		}
		for i := range profiles {
			profile := &profiles[i]
			userDataDir := a.browserMgr.ResolveUserDataDir(profile)
			if _, err := os.Stat(userDataDir); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("读取用户数据目录失败: %w", err)
			}
			added, err := writeProfilePackageDir(zipWriter, userDataDir, "user-data/"+profile.ProfileId)
			if err != nil {
				return fmt.Errorf("导出用户数据失败 [%s]: %w", profile.ProfileName, err)
			}
			fileCount += added
		}
		return nil
	}()

	closeZipErr := zipWriter.Close()
	syncFileErr := out.Sync()
	closeFileErr := out.Close()
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return 0, 0, warnings, writeErr
	}
	if closeZipErr != nil {
		_ = os.Remove(tmpPath)
		return 0, 0, warnings, closeZipErr
	}
	if syncFileErr != nil {
		_ = os.Remove(tmpPath)
		return 0, 0, warnings, syncFileErr
	}
	if closeFileErr != nil {
		_ = os.Remove(tmpPath)
		return 0, 0, warnings, closeFileErr
	}
	if err := fsutil.ReplaceFile(tmpPath, zipPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, 0, warnings, fmt.Errorf("保存导出文件失败: %w", err)
	}
	if _, err := backupWriteProfileMetadata(zipPath, manifest, fileCount, a.appName(), a.appVersion()); err != nil {
		message := fmt.Sprintf("实例包已导出，但元数据写入失败: %v", err)
		warnings = append(warnings, message)
		logger.New("Backup").Warn("实例包已导出，但元数据写入失败", logger.F("path", zipPath), logger.F("error", err.Error()))
	}
	return fileCount, len(portableProfiles), warnings, nil
}

func createProfilePackageTempFile(zipPath string) (*os.File, string, error) {
	targetDir := filepath.Dir(zipPath)
	base := filepath.Base(zipPath)
	temporary, err := os.CreateTemp(targetDir, "."+base+".tmp-*")
	if err != nil {
		return nil, "", err
	}
	return temporary, temporary.Name(), nil
}

func (a *App) importProfilePackageFromPath(zipPath string) (ProfilePackageImportResult, error) {
	return a.importProfilePackageFromPathWithMode(zipPath, profilePackageImportModeNew)
}

func openProfilePackageContents(zipPath string) (*profilePackageContents, error) {
	zipPath = strings.TrimSpace(zipPath)
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打开实例包失败: %w", err)
	}
	contents := &profilePackageContents{Reader: reader}
	completed := false
	defer func() {
		if !completed {
			_ = reader.Close()
		}
	}()
	if err := validateProfilePackageArchive(zipPath, reader.File); err != nil {
		return nil, err
	}

	var manifest ProfilePackageManifest
	if err := readProfilePackageJSON(reader.File, "manifest.json", &manifest); err != nil {
		return nil, err
	}
	if manifest.Format != profilePackageFormat || manifest.Version < 1 || manifest.Version > profilePackageVersion {
		return nil, fmt.Errorf("不支持的实例包格式")
	}
	contents.Manifest = manifest
	var profiles []browser.Profile
	if manifest.Version >= profilePackageDatabaseVersionIntroduced {
		var snapshot ProfilePackageDatabase
		if err := readProfilePackageJSON(reader.File, profilePackageDatabasePath, &snapshot); err != nil {
			return nil, err
		}
		if snapshot.Format != profilePackageDatabaseFormat || snapshot.Version != profilePackageDatabaseVersion {
			return nil, fmt.Errorf("不支持的实例数据库快照格式")
		}
		contents.DatabaseSnapshot = &snapshot
		profiles = snapshot.Profiles
	} else if err := readProfilePackageJSON(reader.File, "profiles.json", &profiles); err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("实例包为空")
	}
	contents.Profiles = profiles
	completed = true
	return contents, nil
}

func validateProfilePackageArchive(zipPath string, files []*zip.File) error {
	info, err := os.Stat(zipPath)
	if err != nil {
		return fmt.Errorf("读取实例包失败: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("实例包不是普通文件")
	}
	if info.Size() > profilePackageMaxCompressedBytes {
		return fmt.Errorf("实例包过大: %d > %d", info.Size(), profilePackageMaxCompressedBytes)
	}
	if len(files) > profilePackageMaxEntries {
		return fmt.Errorf("实例包条目过多: %d > %d", len(files), profilePackageMaxEntries)
	}

	seen := make(map[string]struct{}, len(files))
	var expandedBytes uint64
	for _, file := range files {
		name := strings.TrimSpace(strings.ReplaceAll(file.Name, "\\", "/"))
		if name == "" || strings.IndexByte(name, 0) >= 0 {
			return fmt.Errorf("实例包包含空路径或非法字符")
		}
		cleanName := path.Clean(name)
		if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("实例包包含非法路径: %s", file.Name)
		}
		if _, exists := seen[cleanName]; exists {
			return fmt.Errorf("实例包包含重复路径: %s", cleanName)
		}
		seen[cleanName] = struct{}{}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("实例包不允许包含符号链接: %s", cleanName)
		}
		if file.UncompressedSize64 > profilePackageMaxSingleFileBytes {
			return fmt.Errorf("实例包文件过大(%s): %d > %d", cleanName, file.UncompressedSize64, profilePackageMaxSingleFileBytes)
		}
		if file.UncompressedSize64 > profilePackageMaxExpandedBytes-expandedBytes {
			return fmt.Errorf("实例包解压后总大小超过限制")
		}
		expandedBytes += file.UncompressedSize64
	}
	return nil
}

func (a *App) prepareProfilePackageImportFromPath(zipPath string) (ProfilePackageImportPreview, error) {
	zipPath = strings.TrimSpace(zipPath)
	if zipPath == "" {
		return ProfilePackageImportPreview{}, fmt.Errorf("实例包路径为空")
	}
	contents, err := openProfilePackageContents(zipPath)
	if err != nil {
		return ProfilePackageImportPreview{}, err
	}
	defer contents.Reader.Close()
	if contents.DatabaseSnapshot != nil && (a.db == nil || a.db.GetConn() == nil) {
		return ProfilePackageImportPreview{}, fmt.Errorf("数据库未初始化，无法恢复实例关联数据")
	}

	profiles, conflicts, canOverwrite, err := a.profilePackageImportPreviewProfiles(contents.Profiles)
	if err != nil {
		return ProfilePackageImportPreview{}, err
	}
	portableLoginCount := 0
	for _, profile := range contents.Profiles {
		if _, exists, err := readProfilePackagePortableLoginEnvelope(contents.Reader.File, profile.ProfileId); err != nil {
			return ProfilePackageImportPreview{}, err
		} else if exists {
			portableLoginCount++
		}
	}
	return ProfilePackageImportPreview{
		Cancelled:                 false,
		ZipPath:                   zipPath,
		ProfileCount:              len(contents.Profiles),
		ConflictCount:             len(conflicts),
		CanOverwrite:              canOverwrite,
		Profiles:                  profiles,
		Conflicts:                 conflicts,
		PortableLogin:             portableLoginCount > 0,
		PortableLoginCount:        portableLoginCount,
		RequiresMigrationPassword: portableLoginCount > 0,
		Message:                   profilePackageImportPreviewMessage(profiles, conflicts),
	}, nil
}

func (a *App) profilePackageImportConflicts(profiles []browser.Profile) ([]ProfilePackageImportConflict, bool, error) {
	_, conflicts, canOverwrite, err := a.profilePackageImportPreviewProfiles(profiles)
	return conflicts, canOverwrite, err
}

func (a *App) profilePackageImportPreviewProfiles(profiles []browser.Profile) ([]ProfilePackageImportPreviewProfile, []ProfilePackageImportConflict, bool, error) {
	existing, err := a.loadExistingProfilePackageProfiles()
	if err != nil {
		return nil, nil, false, err
	}
	previewProfiles := make([]ProfilePackageImportPreviewProfile, 0, len(profiles))
	conflicts := make([]ProfilePackageImportConflict, 0)
	canOverwrite := true
	reservedNames := make(map[string]struct{}, len(existing.ByName)+len(profiles))
	for nameKey := range existing.ByName {
		reservedNames[nameKey] = struct{}{}
	}
	seenSourceIDs := make(map[string]struct{}, len(profiles))
	claimedTargets := make(map[string]string, len(profiles))
	sourceImportNameCounts := make(map[string]int, len(profiles))
	for _, source := range profiles {
		sourceImportNameCounts[backupImportTextKey(normalizeImportedProfileName(source.ProfileName))]++
	}
	for index, source := range profiles {
		sourceID := strings.TrimSpace(source.ProfileId)
		if sourceID != "" {
			key := backupImportIDKey(sourceID)
			if _, exists := seenSourceIDs[key]; exists {
				return nil, nil, false, fmt.Errorf("实例包包含重复 profileId: %s", sourceID)
			}
			seenSourceIDs[key] = struct{}{}
		}
		match := findProfilePackageConflict(existing, source, sourceID)
		sourceNameKey := backupImportTextKey(normalizeImportedProfileName(source.ProfileName))
		sourceNameCollision := sourceImportNameCounts[sourceNameKey] > 1
		sourceKey := backupImportIDKey(sourceID)
		if sourceKey == "" {
			sourceKey = fmt.Sprintf("index:%d", index)
		}
		targetCollision := false
		if match.Target != nil {
			targetKey := backupImportIDKey(match.Target.ProfileId)
			if previousSource, claimed := claimedTargets[targetKey]; claimed && previousSource != sourceKey {
				targetCollision = true
			} else {
				claimedTargets[targetKey] = sourceKey
			}
		}
		conflict := ProfilePackageImportConflict{
			SourceProfileID:       sourceID,
			SourceProfileName:     strings.TrimSpace(source.ProfileName),
			MatchType:             match.MatchType,
			Ambiguous:             match.Ambiguous,
			TargetMatches:         len(match.TargetHits),
			SourceTargetCollision: targetCollision,
			SourceNameCollision:   sourceNameCollision,
		}
		if sourceNameCollision && conflict.TargetMatches == 0 {
			conflict.TargetMatches = sourceImportNameCounts[sourceNameKey]
		}
		if match.Target != nil {
			conflict.TargetProfileID = match.Target.ProfileId
			conflict.TargetProfileName = match.Target.ProfileName
			conflict.TargetRunning = profilePackageProfileIsRunning(a, *match.Target)
			conflict.TargetDeleted = strings.TrimSpace(match.Target.DeletedAt) != ""
		}
		if conflict.Ambiguous || conflict.TargetRunning || conflict.SourceTargetCollision || conflict.SourceNameCollision {
			canOverwrite = false
		}
		if match.Target != nil && (a == nil || a.db == nil || a.db.GetConn() == nil) {
			if match.Target != nil {
				canOverwrite = false
			}
		}
		conflictPresent := match.Target != nil || match.Ambiguous || sourceNameCollision
		rowCanOverwrite := conflictPresent && match.Target != nil && !conflict.Ambiguous && !conflict.TargetRunning && !conflict.SourceTargetCollision && !conflict.SourceNameCollision && a != nil && a.db != nil && a.db.GetConn() != nil
		row := ProfilePackageImportPreviewProfile{
			SourceIndex:           index,
			SourceProfileID:       sourceID,
			SourceProfileName:     strings.TrimSpace(source.ProfileName),
			MatchType:             match.MatchType,
			TargetRunning:         conflict.TargetRunning,
			TargetDeleted:         conflict.TargetDeleted,
			Ambiguous:             conflict.Ambiguous,
			TargetMatches:         conflict.TargetMatches,
			SourceTargetCollision: conflict.SourceTargetCollision,
			SourceNameCollision:   conflict.SourceNameCollision,
			SuggestedAction:       profilePackageImportModeNew,
			CanOverwrite:          rowCanOverwrite,
		}
		if match.Target != nil {
			row.TargetProfileID = match.Target.ProfileId
			row.TargetProfileName = match.Target.ProfileName
		}
		row.SuggestedProfileName = uniqueImportedProfileName(source.ProfileName, reservedNames)
		if rowCanOverwrite {
			row.SuggestedAction = profilePackageImportModeOverwrite
		}
		previewProfiles = append(previewProfiles, row)
		if conflictPresent {
			if !rowCanOverwrite {
				canOverwrite = false
			}
			conflicts = append(conflicts, conflict)
		}
	}
	return previewProfiles, conflicts, canOverwrite, nil
}

func (a *App) importProfilePackageFromPathWithMode(zipPath string, mode string) (ProfilePackageImportResult, error) {
	return a.importProfilePackageFromPathWithModeAndConfirmation(zipPath, mode, true)
}

func (a *App) importProfilePackageFromPathWithModeAndConfirmation(zipPath string, mode string, confirmConflict bool) (ProfilePackageImportResult, error) {
	return a.importProfilePackageFromPathWithModeAndActions(zipPath, mode, confirmConflict, nil)
}

func (a *App) importProfilePackageFromPathWithModeAndActions(zipPath string, mode string, confirmConflict bool, actions []ProfilePackageImportAction) (ProfilePackageImportResult, error) {
	return a.importProfilePackageFromPathWithModeAndActionsAndPassword(zipPath, mode, confirmConflict, actions, "")
}

func (a *App) importProfilePackageFromPathWithModeAndActionsAndPassword(zipPath string, mode string, confirmConflict bool, actions []ProfilePackageImportAction, migrationPassword string) (result ProfilePackageImportResult, retErr error) {
	mode = normalizeProfilePackageImportMode(mode)
	if mode == "" {
		return ProfilePackageImportResult{}, fmt.Errorf("不支持的实例导入冲突处理方式")
	}
	contents, err := openProfilePackageContents(zipPath)
	if err != nil {
		return ProfilePackageImportResult{}, err
	}
	defer contents.Reader.Close()
	if contents.DatabaseSnapshot != nil && (a.db == nil || a.db.GetConn() == nil) {
		return ProfilePackageImportResult{}, fmt.Errorf("数据库未初始化，无法恢复实例关联数据")
	}
	if a.browserMgr == nil {
		return ProfilePackageImportResult{}, fmt.Errorf("浏览器管理器未初始化")
	}
	a.browserMgr.InitData()
	existing, err := a.loadExistingProfilePackageProfiles()
	if err != nil {
		return ProfilePackageImportResult{}, err
	}
	hasExplicitActions := len(actions) > 0
	actionsByIndex := make(map[int]ProfilePackageImportAction, len(actions))
	if hasExplicitActions {
		if len(actions) != len(contents.Profiles) {
			return ProfilePackageImportResult{}, fmt.Errorf("实例处理表不完整，请为每个实例选择处理方式")
		}
		for _, action := range actions {
			if action.SourceIndex < 0 || action.SourceIndex >= len(contents.Profiles) {
				return ProfilePackageImportResult{}, fmt.Errorf("实例处理表包含无效的实例行")
			}
			if _, exists := actionsByIndex[action.SourceIndex]; exists {
				return ProfilePackageImportResult{}, fmt.Errorf("实例处理表包含重复的实例行")
			}
			sourceID := strings.TrimSpace(contents.Profiles[action.SourceIndex].ProfileId)
			if strings.TrimSpace(action.SourceProfileID) != "" && !strings.EqualFold(strings.TrimSpace(action.SourceProfileID), sourceID) {
				return ProfilePackageImportResult{}, fmt.Errorf("实例处理表与备份实例不一致")
			}
			action.Mode = normalizeProfilePackageImportAction(action.Mode)
			if action.Mode == "" {
				return ProfilePackageImportResult{}, fmt.Errorf("实例处理表包含不支持的处理方式")
			}
			if action.Mode == profilePackageImportModeRename && strings.TrimSpace(action.ProfileName) == "" {
				return ProfilePackageImportResult{}, fmt.Errorf("请选择重命名后的实例名称")
			}
			actionsByIndex[action.SourceIndex] = action
		}
	}
	if hasExplicitActions && !confirmConflict {
		return ProfilePackageImportResult{}, fmt.Errorf("实例处理方式尚未确认")
	}
	if !confirmConflict {
		conflicts, _, err := a.profilePackageImportConflicts(contents.Profiles)
		if err != nil {
			return ProfilePackageImportResult{}, err
		}
		if len(conflicts) > 0 {
			return ProfilePackageImportResult{}, fmt.Errorf("检测到实例同名或 ID 冲突，请先在冲突提示中选择覆盖原有或创建新的")
		}
	}
	now := time.Now().Format(time.RFC3339)
	mappings := make(map[string]string, len(contents.Profiles))
	warnings := make([]string, 0)
	if contents.DatabaseSnapshot != nil {
		warnings = append(warnings, contents.DatabaseSnapshot.Warnings...)
	}
	usedIDs := make(map[string]struct{}, len(existing.ByID)+len(contents.Profiles))
	reservedNames := make(map[string]struct{}, len(existing.ByName)+len(contents.Profiles))
	for key := range existing.ByID {
		usedIDs[key] = struct{}{}
	}
	for key := range existing.UsedIDs {
		usedIDs[key] = struct{}{}
	}
	for key := range existing.ByName {
		reservedNames[key] = struct{}{}
	}

	prepared := make([]preparedProfilePackageImport, 0, len(contents.Profiles))
	finalNameKeys := make(map[string]struct{}, len(contents.Profiles))
	batchID := uuid.NewString()
	stagingRoot := a.profilePackageImportStagingRoot(batchID)
	swaps := make([]profilePackageDirectorySwap, 0, len(contents.Profiles))
	originalProfiles := make(map[string]*browser.Profile, len(contents.Profiles))
	committed := false
	defer func() {
		_ = os.RemoveAll(stagingRoot)
		if committed {
			return
		}
		rollbackErr := rollbackProfilePackageDirectorySwaps(swaps)
		if len(originalProfiles) > 0 {
			a.browserMgr.Mutex.Lock()
			for profileID, original := range originalProfiles {
				if original == nil {
					delete(a.browserMgr.Profiles, profileID)
					continue
				}
				copyProfile := *original
				a.browserMgr.Profiles[profileID] = &copyProfile
			}
			a.browserMgr.Mutex.Unlock()
		}
		retErr = combineProfilePackageRollbackError(retErr, rollbackErr)
	}()

	seenSourceIDs := make(map[string]struct{}, len(contents.Profiles))
	claimedTargets := make(map[string]string, len(contents.Profiles))
	sourceImportNameCounts := make(map[string]int, len(contents.Profiles))
	for _, source := range contents.Profiles {
		sourceImportNameCounts[backupImportTextKey(normalizeImportedProfileName(source.ProfileName))]++
	}
	for index, source := range contents.Profiles {
		oldID := strings.TrimSpace(source.ProfileId)
		if oldID == "" {
			oldID = uuid.NewString()
		}
		if key := backupImportIDKey(oldID); key != "" {
			if _, exists := seenSourceIDs[key]; exists {
				return ProfilePackageImportResult{}, fmt.Errorf("实例包包含重复 profileId: %s", oldID)
			}
			seenSourceIDs[key] = struct{}{}
		}
		match := findProfilePackageConflict(existing, source, oldID)
		sourceNameCollision := sourceImportNameCounts[backupImportTextKey(normalizeImportedProfileName(source.ProfileName))] > 1
		actionMode := mode
		requestedProfileName := ""
		if action, exists := actionsByIndex[index]; exists {
			actionMode = action.Mode
			requestedProfileName = strings.TrimSpace(action.ProfileName)
		}
		if actionMode == profilePackageImportModeOverwrite {
			if sourceNameCollision {
				return ProfilePackageImportResult{}, fmt.Errorf("实例包内存在多个同名实例，无法自动覆盖，请选择新建")
			}
			if match.Ambiguous {
				return ProfilePackageImportResult{}, fmt.Errorf("实例「%s」存在多个同名目标，无法自动覆盖，请选择新建", source.ProfileName)
			}
			if match.Target != nil && profilePackageProfileIsRunning(a, *match.Target) {
				return ProfilePackageImportResult{}, fmt.Errorf("目标实例「%s」正在运行，请先停止后再覆盖", match.Target.ProfileName)
			}
			if match.Target != nil {
				targetKey := backupImportIDKey(match.Target.ProfileId)
				sourceKey := backupImportIDKey(oldID)
				if sourceKey == "" {
					sourceKey = fmt.Sprintf("index:%d", index)
				}
				if previousSource, claimed := claimedTargets[targetKey]; claimed && previousSource != sourceKey {
					return ProfilePackageImportResult{}, fmt.Errorf("多个导入实例匹配目标实例「%s」，无法自动覆盖，请选择新建", match.Target.ProfileName)
				}
				claimedTargets[targetKey] = sourceKey
			}
			if match.Target != nil && (a.db == nil || a.db.GetConn() == nil) {
				return ProfilePackageImportResult{}, fmt.Errorf("当前环境不支持覆盖并备份到回收站")
			}
			if hasExplicitActions && match.Target == nil {
				return ProfilePackageImportResult{}, fmt.Errorf("实例「%s」没有可覆盖的目标，请改为新建或重命名", source.ProfileName)
			}
		}
		if actionMode == profilePackageImportModeRename {
			if requestedProfileName == "" {
				return ProfilePackageImportResult{}, fmt.Errorf("请选择重命名后的实例名称")
			}
			nameKey := backupImportTextKey(requestedProfileName)
			if _, exists := reservedNames[nameKey]; exists {
				return ProfilePackageImportResult{}, fmt.Errorf("重命名后的实例「%s」已存在，请换一个名称", requestedProfileName)
			}
			reservedNames[nameKey] = struct{}{}
		}

		newID := profilePackageGeneratedID(usedIDs)
		overwrite := actionMode == profilePackageImportModeOverwrite && match.Target != nil
		profile := source
		if overwrite {
			profile.ProfileName = strings.TrimSpace(source.ProfileName)
			if profile.ProfileName == "" {
				profile.ProfileName = match.Target.ProfileName
			}
			profile.ProfileName = uniqueImportedProfileNameForOverwrite(profile.ProfileName, *match.Target, existing, reservedNames)
			profile.UserDataDir = newID
			profile.CreatedAt = strings.TrimSpace(match.Target.CreatedAt)
		} else if actionMode == profilePackageImportModeRename {
			profile.ProfileName = requestedProfileName
			profile.UserDataDir = newID
		} else {
			profile.ProfileName = uniqueImportedProfileName(source.ProfileName, reservedNames)
			profile.UserDataDir = newID
		}
		if profile.ProfileName == "" {
			profile.ProfileName = "导入实例"
		}
		finalNameKey := backupImportTextKey(profile.ProfileName)
		if _, exists := finalNameKeys[finalNameKey]; exists {
			return ProfilePackageImportResult{}, fmt.Errorf("导入后的实例名称重复：%s，请调整重命名操作", profile.ProfileName)
		}
		finalNameKeys[finalNameKey] = struct{}{}
		profile.ProfileId = newID
		profile.RuntimeState = browser.RuntimeStopped
		profile.Running = false
		profile.DebugPort = 0
		profile.DebugReady = false
		profile.Pid = 0
		profile.RuntimeWarning = ""
		profile.LastError = ""
		profile.LaunchCode = ""
		if strings.TrimSpace(profile.CreatedAt) == "" {
			profile.CreatedAt = now
		}
		profile.UpdatedAt = now
		profile.DeletedAt = ""
		if contents.DatabaseSnapshot == nil {
			if warning := a.applyImportedProfileProxyByName(&profile); warning != "" {
				warnings = append(warnings, fmt.Sprintf("实例「%s」%s", profile.ProfileName, warning))
			}
		}

		profileRef := &browser.Profile{ProfileId: newID, UserDataDir: profile.UserDataDir}
		finalDir := a.browserMgr.ResolveUserDataDir(profileRef)
		stagingDir := filepath.Join(stagingRoot, newID)
		hasUserData, err := a.extractProfileUserDataToDir(contents.Reader.File, oldID, stagingDir)
		if err != nil {
			return ProfilePackageImportResult{}, err
		}
		if !hasUserData {
			warnings = append(warnings, fmt.Sprintf("实例「%s」没有用户数据目录，仅导入配置", profile.ProfileName))
		}
		envelope, hasPortableLogin, err := readProfilePackagePortableLoginEnvelope(contents.Reader.File, oldID)
		if err != nil {
			return ProfilePackageImportResult{}, err
		}
		if hasPortableLogin {
			if !hasUserData {
				warnings = append(warnings, fmt.Sprintf("实例「%s」包含登录态迁移数据，但没有用户数据目录，已跳过登录态恢复", profile.ProfileName))
			} else if strings.TrimSpace(migrationPassword) == "" {
				warnings = append(warnings, fmt.Sprintf("实例「%s」包含可迁移登录态，但未提供迁移密码；跨电脑登录状态可能不可用", profile.ProfileName))
			} else if err := restoreProfilePortableLoginEnvelope(stagingDir, migrationPassword, envelope); err != nil {
				if err == errPortableLoginBadPassword {
					return ProfilePackageImportResult{}, fmt.Errorf("登录态迁移密码不正确")
				}
				return ProfilePackageImportResult{}, fmt.Errorf("恢复实例登录态失败 [%s]: %w", profile.ProfileName, err)
			}
		}
		replacedProfileID := ""
		if overwrite && match.Target != nil {
			replacedProfileID = strings.TrimSpace(match.Target.ProfileId)
		}
		prepared = append(prepared, preparedProfilePackageImport{
			Profile:           profile,
			OldProfileID:      oldID,
			ReplacedProfileID: replacedProfileID,
			FinalDir:          finalDir,
			StagingDir:        stagingDir,
			HasUserData:       hasUserData,
			Action:            actionMode,
			Overwrite:         overwrite,
		})
		mappings[oldID] = newID
	}

	for _, item := range prepared {
		a.browserMgr.Mutex.Lock()
		for _, profileID := range []string{item.Profile.ProfileId, item.ReplacedProfileID} {
			profileID = strings.TrimSpace(profileID)
			if profileID == "" {
				continue
			}
			if current, exists := a.browserMgr.Profiles[profileID]; exists && current != nil {
				copyProfile := *current
				originalProfiles[profileID] = &copyProfile
			} else if _, recorded := originalProfiles[profileID]; !recorded {
				originalProfiles[profileID] = nil
			}
		}
		a.browserMgr.Mutex.Unlock()
	}
	preparedExtensionArtifacts := make([]preparedProfilePackageExtensionArtifact, 0)
	if contents.DatabaseSnapshot != nil {
		preparedExtensionArtifacts, err = a.prepareProfilePackageExtensionArtifacts(
			contents.Reader.File,
			contents.DatabaseSnapshot,
			stagingRoot,
			&warnings,
		)
		if err != nil {
			return ProfilePackageImportResult{}, err
		}
	}
	for _, item := range prepared {
		if !item.HasUserData {
			continue
		}
		swap, err := replaceProfileUserDataDirWithBackup(item.StagingDir, item.FinalDir)
		if err != nil {
			return ProfilePackageImportResult{}, err
		}
		swaps = append(swaps, swap)
	}
	if err := applyPreparedProfilePackageExtensionArtifacts(preparedExtensionArtifacts, &swaps); err != nil {
		return ProfilePackageImportResult{}, err
	}
	legacyDatabaseRestore := contents.DatabaseSnapshot == nil && a.db != nil && a.db.GetConn() != nil
	if contents.DatabaseSnapshot != nil {
		if err := a.restoreProfilePackageDatabase(*contents.DatabaseSnapshot, prepared, &warnings); err != nil {
			return ProfilePackageImportResult{}, err
		}
	} else if legacyDatabaseRestore {
		if err := a.restoreLegacyProfilePackageDatabase(prepared); err != nil {
			return ProfilePackageImportResult{}, err
		}
	}
	a.browserMgr.Mutex.Lock()
	for i := range prepared {
		profile := &prepared[i].Profile
		if replacedID := strings.TrimSpace(prepared[i].ReplacedProfileID); replacedID != "" {
			delete(a.browserMgr.Profiles, replacedID)
		}
		a.browserMgr.Profiles[profile.ProfileId] = profile
		if a.launchCodeSvc != nil {
			if code, err := a.launchCodeSvc.EnsureCode(profile.ProfileId); err == nil {
				profile.LaunchCode = code
			}
		}
	}
	a.browserMgr.Mutex.Unlock()
	if contents.DatabaseSnapshot == nil && !legacyDatabaseRestore {
		if err := a.browserMgr.SaveProfiles(); err != nil {
			return ProfilePackageImportResult{}, err
		}
	}
	finalizeProfilePackageDirectorySwaps(swaps)
	committed = true
	createdCount := 0
	overwrittenCount := 0
	renameCount := 0
	for _, item := range prepared {
		if item.Overwrite {
			overwrittenCount++
		} else if item.Action == profilePackageImportModeRename {
			renameCount++
		} else {
			createdCount++
		}
	}
	return ProfilePackageImportResult{
		Cancelled:        false,
		ImportedCount:    len(prepared),
		CreatedCount:     createdCount,
		OverwrittenCount: overwrittenCount,
		RenamedCount:     renameCount,
		ProfileMappings:  mappings,
		Warnings:         warnings,
		Message:          "导入完成",
	}, nil
}

func normalizeProfilePackageImportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", profilePackageImportModeNew:
		return profilePackageImportModeNew
	case profilePackageImportModeOverwrite:
		return profilePackageImportModeOverwrite
	case profilePackageImportModeRename:
		return profilePackageImportModeRename
	default:
		return ""
	}
}

func normalizeProfilePackageImportAction(mode string) string {
	return normalizeProfilePackageImportMode(mode)
}

func profilePackageImportPreviewMessage(profiles []ProfilePackageImportPreviewProfile, conflicts []ProfilePackageImportConflict) string {
	if len(conflicts) == 0 {
		return fmt.Sprintf("共 %d 个实例，未发现冲突", len(profiles))
	}
	return fmt.Sprintf("共 %d 个实例，发现 %d 个冲突，请逐项确认处理方式", len(profiles), len(conflicts))
}

func (a *App) loadExistingProfilePackageProfiles() (profilePackageExistingProfiles, error) {
	if a == nil || a.browserMgr == nil {
		return profilePackageExistingProfiles{}, fmt.Errorf("浏览器管理器未初始化")
	}
	a.browserMgr.InitData()
	existing := profilePackageExistingProfiles{
		ByID:    make(map[string]browser.Profile),
		ByName:  make(map[string][]browser.Profile),
		UsedIDs: make(map[string]struct{}),
	}
	addProfile := func(profile browser.Profile, active bool) {
		idKey := backupImportIDKey(profile.ProfileId)
		if idKey == "" {
			return
		}
		existing.UsedIDs[idKey] = struct{}{}
		if !active || strings.TrimSpace(profile.DeletedAt) != "" {
			return
		}
		if _, exists := existing.ByID[idKey]; !exists {
			existing.ByID[idKey] = profile
		}
		nameKey := backupImportTextKey(profile.ProfileName)
		if nameKey == "" {
			return
		}
		for _, item := range existing.ByName[nameKey] {
			if backupImportIDKey(item.ProfileId) == idKey {
				return
			}
		}
		existing.ByName[nameKey] = append(existing.ByName[nameKey], profile)
	}
	a.browserMgr.Mutex.Lock()
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		addProfile(*profile, true)
	}
	profileDAO := a.browserMgr.ProfileDAO
	a.browserMgr.Mutex.Unlock()
	if profileDAO != nil {
		deleted, err := profileDAO.ListDeleted()
		if err != nil {
			return profilePackageExistingProfiles{}, fmt.Errorf("读取回收站实例失败: %w", err)
		}
		for _, profile := range deleted {
			if profile != nil {
				addProfile(*profile, false)
			}
		}
	}
	return existing, nil
}

func findProfilePackageConflict(existing profilePackageExistingProfiles, source browser.Profile, sourceID string) profilePackageConflictMatch {
	if idKey := backupImportIDKey(sourceID); idKey != "" {
		if target, exists := existing.ByID[idKey]; exists {
			copyProfile := target
			return profilePackageConflictMatch{
				Target:     &copyProfile,
				MatchType:  profilePackageImportMatchID,
				TargetHits: []browser.Profile{target},
			}
		}
	}
	hits := existing.ByName[backupImportTextKey(normalizeImportedProfileName(source.ProfileName))]
	if len(hits) == 1 {
		copyProfile := hits[0]
		return profilePackageConflictMatch{
			Target:     &copyProfile,
			MatchType:  profilePackageImportMatchName,
			TargetHits: hits,
		}
	}
	if len(hits) > 1 {
		return profilePackageConflictMatch{
			MatchType:  profilePackageImportMatchName,
			Ambiguous:  true,
			TargetHits: hits,
		}
	}
	return profilePackageConflictMatch{}
}

func profilePackageProfileIsRunning(a *App, profile browser.Profile) bool {
	if profile.Running || profile.Pid > 0 || profile.DebugPort > 0 || profile.WindowMarkerCode != "" {
		return true
	}
	if a == nil || a.browserMgr == nil {
		return false
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	return a.browserMgr.BrowserProcesses[profile.ProfileId] != nil
}

func uniqueImportedProfileName(name string, reserved map[string]struct{}) string {
	base := normalizeImportedProfileName(name)
	if _, exists := reserved[backupImportTextKey(base)]; !exists {
		reserved[backupImportTextKey(base)] = struct{}{}
		return base
	}
	candidate := buildImportedProfileName(base)
	if _, exists := reserved[backupImportTextKey(candidate)]; !exists {
		reserved[backupImportTextKey(candidate)] = struct{}{}
		return candidate
	}
	for index := 2; ; index++ {
		candidate = fmt.Sprintf("%s（导入 %d）", base, index)
		if _, exists := reserved[backupImportTextKey(candidate)]; exists {
			continue
		}
		reserved[backupImportTextKey(candidate)] = struct{}{}
		return candidate
	}
}

func uniqueImportedProfileNameForOverwrite(name string, target browser.Profile, existing profilePackageExistingProfiles, reserved map[string]struct{}) string {
	base := normalizeImportedProfileName(name)
	nameKey := backupImportTextKey(base)
	targetKey := backupImportIDKey(target.ProfileId)
	for _, profile := range existing.ByName[nameKey] {
		if backupImportIDKey(profile.ProfileId) != targetKey {
			return uniqueImportedProfileName(base, reserved)
		}
	}
	reserved[nameKey] = struct{}{}
	return base
}

func (a *App) extractProfileUserDataToDir(files []*zip.File, oldProfileID string, destDir string) (bool, error) {
	prefix := "user-data/" + oldProfileID + "/"
	hasUserData := false
	for _, file := range files {
		name := filepath.ToSlash(file.Name)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}
		if !hasUserData {
			if err := os.RemoveAll(destDir); err != nil {
				return false, fmt.Errorf("清理临时用户数据目录失败: %w", err)
			}
			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return false, fmt.Errorf("创建临时用户数据目录失败: %w", err)
			}
			hasUserData = true
		}
		if err := extractProfilePackageFile(file, destDir, rel); err != nil {
			return false, err
		}
	}
	return hasUserData, nil
}

func writeProfilePackageJSON(zipWriter *zip.Writer, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	writer, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func writeProfilePackageDir(zipWriter *zip.Writer, srcDir string, destPrefix string) (int, error) {
	count := 0
	err := filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		zipName := filepath.ToSlash(filepath.Join(destPrefix, rel))
		if entry.IsDir() {
			_, err := zipWriter.Create(strings.TrimSuffix(zipName, "/") + "/")
			return err
		}
		writer, err := zipWriter.Create(zipName)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count++
		return nil
	})
	return count, err
}

func readProfilePackageJSON(files []*zip.File, name string, target any) error {
	for _, file := range files {
		if filepath.ToSlash(file.Name) != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		defer reader.Close()
		return json.NewDecoder(reader).Decode(target)
	}
	return fmt.Errorf("实例包缺少 %s", name)
}

func readProfilePackagePortableLoginEnvelope(files []*zip.File, profileID string) (profilePortableLoginEnvelope, bool, error) {
	name := profilePortableLoginEntryName(profileID)
	for _, file := range files {
		if filepath.ToSlash(file.Name) != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return profilePortableLoginEnvelope{}, false, err
		}
		defer reader.Close()
		var envelope profilePortableLoginEnvelope
		if err := json.NewDecoder(reader).Decode(&envelope); err != nil {
			return profilePortableLoginEnvelope{}, false, fmt.Errorf("解析登录态迁移数据失败: %w", err)
		}
		return envelope, true, nil
	}
	return profilePortableLoginEnvelope{}, false, nil
}

func extractProfilePackageFile(file *zip.File, destDir string, rel string) error {
	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return fmt.Errorf("非法路径: %s", rel)
	}
	target := filepath.Join(destDir, cleanRel)
	cleanDest := filepath.Clean(destDir)
	cleanTarget := filepath.Clean(target)
	if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
		return fmt.Errorf("非法路径: %s", rel)
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, reader)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func replaceProfileUserDataDir(stagingDir string, finalDir string) error {
	swap, err := replaceProfileUserDataDirWithBackup(stagingDir, finalDir)
	if err != nil {
		return err
	}
	finalizeProfilePackageDirectorySwaps([]profilePackageDirectorySwap{swap})
	return nil
}

func replaceProfileUserDataDirWithBackup(stagingDir string, finalDir string) (profilePackageDirectorySwap, error) {
	if strings.TrimSpace(stagingDir) == "" || strings.TrimSpace(finalDir) == "" {
		return profilePackageDirectorySwap{}, fmt.Errorf("用户数据目录不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return profilePackageDirectorySwap{}, fmt.Errorf("创建用户数据父目录失败: %w", err)
	}
	backupDir := finalDir + ".profile-package-backup-" + uuid.NewString()
	swap := profilePackageDirectorySwap{
		FinalDir:  finalDir,
		BackupDir: backupDir,
	}
	if _, err := os.Stat(finalDir); err == nil {
		swap.HadOriginal = true
		if err := os.Rename(finalDir, backupDir); err != nil {
			return profilePackageDirectorySwap{}, fmt.Errorf("备份现有用户数据目录失败: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return profilePackageDirectorySwap{}, fmt.Errorf("检查用户数据目录失败: %w", err)
	}
	if err := os.Rename(stagingDir, finalDir); err != nil {
		if swap.HadOriginal {
			if restoreErr := os.Rename(backupDir, finalDir); restoreErr != nil {
				return profilePackageDirectorySwap{}, fmt.Errorf("提交用户数据目录失败: %w; IMPORT_ROLLBACK_FAILED: 恢复原目录失败: %v", err, restoreErr)
			}
		}
		return profilePackageDirectorySwap{}, fmt.Errorf("提交用户数据目录失败: %w", err)
	}
	return swap, nil
}

func finalizeProfilePackageDirectorySwaps(swaps []profilePackageDirectorySwap) {
	for _, swap := range swaps {
		if !swap.HadOriginal || strings.TrimSpace(swap.BackupDir) == "" {
			continue
		}
		_ = os.RemoveAll(swap.BackupDir)
	}
}

func rollbackProfilePackageDirectorySwaps(swaps []profilePackageDirectorySwap) error {
	rollbackErrors := make([]error, 0)
	for index := len(swaps) - 1; index >= 0; index-- {
		swap := swaps[index]
		if strings.TrimSpace(swap.FinalDir) == "" {
			continue
		}
		if err := os.RemoveAll(swap.FinalDir); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("清理失败导入目录 %s: %w", swap.FinalDir, err))
		}
		if swap.HadOriginal && strings.TrimSpace(swap.BackupDir) != "" {
			if err := os.Rename(swap.BackupDir, swap.FinalDir); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复原用户数据目录 %s: %w", swap.FinalDir, err))
				continue
			}
			if _, err := os.Stat(swap.FinalDir); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("验证恢复后的用户数据目录 %s: %w", swap.FinalDir, err))
			}
		}
	}
	return errors.Join(rollbackErrors...)
}

func combineProfilePackageRollbackError(importErr error, rollbackErr error) error {
	if rollbackErr == nil {
		return importErr
	}
	rollbackFailure := fmt.Errorf("IMPORT_ROLLBACK_FAILED: %w", rollbackErr)
	if importErr == nil {
		return rollbackFailure
	}
	return errors.Join(importErr, rollbackFailure)
}

func (a *App) profilePackageImportStagingRoot(batchID string) string {
	root := "data"
	if a.browserMgr != nil && a.browserMgr.Config != nil {
		root = strings.TrimSpace(a.browserMgr.Config.Browser.UserDataRoot)
	}
	if root == "" {
		root = "data"
	}
	if a.browserMgr != nil {
		root = a.browserMgr.ResolveRelativePath(root)
	} else {
		root = a.resolveAppPath(root)
	}
	return filepath.Join(root, ".imports", strings.TrimSpace(batchID))
}

func normalizeProfilePackageIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func profilePackageProfileNames(profiles []browser.Profile) []string {
	result := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		name := strings.TrimSpace(profile.ProfileName)
		if name == "" {
			name = strings.TrimSpace(profile.ProfileId)
		}
		if name == "" {
			continue
		}
		result = append(result, name)
	}
	return result
}

func ensureZipSuffix(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		return path
	}
	return path + ".zip"
}

func buildImportedProfileName(name string) string {
	return normalizeImportedProfileName(name) + "（导入）"
}

func normalizeImportedProfileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "导入实例"
	}
	return name
}

func (a *App) prepareProfileProxyForPackage(profile *browser.Profile) {
	if profile == nil {
		return
	}
	proxyName := strings.TrimSpace(profile.ProxyBindName)
	if proxyName == "" {
		if proxy, ok := a.browserMgr.GetProxyByID(profile.ProxyId); ok {
			proxyName = strings.TrimSpace(proxy.ProxyName)
		}
	}
	profile.ProxyId = ""
	profile.ProxyConfig = ""
	profile.ProxyBindSourceID = ""
	profile.ProxyBindSourceURL = ""
	profile.ProxyBindName = proxyName
	profile.ProxyBindUpdatedAt = ""
}

func (a *App) applyImportedProfileProxyByName(profile *browser.Profile) string {
	if profile == nil {
		return ""
	}
	proxyName := strings.TrimSpace(profile.ProxyBindName)
	profile.ProxyId = ""
	profile.ProxyConfig = ""
	profile.ProxyBindSourceID = ""
	profile.ProxyBindSourceURL = ""
	profile.ProxyBindUpdatedAt = ""
	if proxyName == "" {
		profile.ProxyBindName = ""
		return ""
	}
	proxy, matchCount := a.findProxiesByName(proxyName)
	if matchCount == 1 {
		browser.BindProfileToProxy(profile, proxy, true)
		return ""
	}
	profile.ProxyBindName = ""
	if matchCount == 0 {
		return fmt.Sprintf("绑定代理「%s」未找到，已清空绑定", proxyName)
	}
	return fmt.Sprintf("绑定代理「%s」存在多个同名匹配，已清空绑定", proxyName)
}

func (a *App) findUniqueProxyByName(proxyName string) (browser.Proxy, bool) {
	proxy, count := a.findProxiesByName(proxyName)
	return proxy, count == 1
}

func (a *App) findProxiesByName(proxyName string) (browser.Proxy, int) {
	target := strings.ToLower(strings.TrimSpace(proxyName))
	if target == "" {
		return browser.Proxy{}, 0
	}
	proxies := browser.ListProxiesWithFallback(a.browserMgr.ProxyDAO, a.config.Browser.Proxies)
	var hit browser.Proxy
	matched := 0
	for _, proxy := range proxies {
		if strings.ToLower(strings.TrimSpace(proxy.ProxyName)) != target {
			continue
		}
		hit = proxy
		matched++
		if matched > 1 {
			return browser.Proxy{}, matched
		}
	}
	return hit, matched
}
