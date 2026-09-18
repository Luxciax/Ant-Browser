package backend

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/browser"
)

const profilePackageExtensionRoot = "extensions"

type profilePackageExtensionArtifact struct {
	ExtensionID string
	SourceDir   string
}

type preparedProfilePackageExtensionArtifact struct {
	ExtensionID string
	StagingDir  string
	FinalDir    string
}

type profilePackageExtensionImportDecision string

const (
	profilePackageExtensionReuse    profilePackageExtensionImportDecision = "reuse"
	profilePackageExtensionUpdate   profilePackageExtensionImportDecision = "update"
	profilePackageExtensionConflict profilePackageExtensionImportDecision = "conflict"
)

func (a *App) collectProfilePackageExtensionArtifacts(snapshot *ProfilePackageDatabase) ([]profilePackageExtensionArtifact, []string) {
	if a == nil || a.browserMgr == nil || snapshot == nil {
		return nil, nil
	}
	artifacts := make([]profilePackageExtensionArtifact, 0, len(snapshot.Extensions))
	warnings := make([]string, 0)
	for index := range snapshot.Extensions {
		extension := &snapshot.Extensions[index]
		extensionID := browser.NormalizeExtensionID(extension.ExtensionID)
		if extensionID == "" {
			warnings = append(warnings, fmt.Sprintf("插件「%s」ID 无效，未打包插件文件", extension.Name))
			continue
		}
		installDir := strings.TrimSpace(extension.InstallDir)
		if installDir != "" && !filepath.IsAbs(installDir) {
			installDir = a.browserMgr.ResolveRelativePath(installDir)
		}
		if installDir == "" || !backupExtensionManifestExists(installDir) {
			warnings = append(warnings, fmt.Sprintf("插件「%s」缺少可迁移安装目录，跨电脑导入后可能需要重新安装", extension.Name))
			continue
		}
		artifacts = append(artifacts, profilePackageExtensionArtifact{ExtensionID: extensionID, SourceDir: installDir})
	}
	return artifacts, warnings
}

func writeProfilePackageExtensionArtifacts(zipWriter *zip.Writer, artifacts []profilePackageExtensionArtifact) (int, error) {
	count := 0
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.SourceDir) == "" || strings.TrimSpace(artifact.ExtensionID) == "" {
			continue
		}
		added, err := writeProfilePackageDir(zipWriter, artifact.SourceDir, filepath.ToSlash(filepath.Join(profilePackageExtensionRoot, artifact.ExtensionID)))
		if err != nil {
			return count, fmt.Errorf("打包插件文件失败 [%s]: %w", artifact.ExtensionID, err)
		}
		count += added
	}
	return count, nil
}

func (a *App) prepareProfilePackageExtensionArtifacts(files []*zip.File, snapshot *ProfilePackageDatabase, stagingRoot string, warnings *[]string) ([]preparedProfilePackageExtensionArtifact, error) {
	if a == nil || a.browserMgr == nil || snapshot == nil {
		return nil, nil
	}
	prepared := make([]preparedProfilePackageExtensionArtifact, 0, len(snapshot.Extensions))
	for index := range snapshot.Extensions {
		extension := &snapshot.Extensions[index]
		extensionID := browser.NormalizeExtensionID(extension.ExtensionID)
		if extensionID == "" {
			appendProfilePackageWarning(warnings, fmt.Sprintf("插件「%s」ID 无效，已跳过插件文件恢复", extension.Name))
			continue
		}

		existing, usable, err := a.profilePackageExistingExtension(extensionID)
		if err != nil {
			return nil, err
		}
		if usable && existing != nil {
			decision, reason := decideProfilePackageExtensionImport(*existing, *extension)
			switch decision {
			case profilePackageExtensionReuse:
				appendProfilePackageWarning(warnings, fmt.Sprintf("插件「%s」已存在，复用目标机版本%s", extension.Name, reason))
				continue
			case profilePackageExtensionConflict:
				return nil, fmt.Errorf("插件「%s」与目标机同 ID 插件冲突%s", extension.Name, reason)
			case profilePackageExtensionUpdate:
				if !profilePackageExtensionArtifactPresent(files, extensionID) {
					appendProfilePackageWarning(warnings, fmt.Sprintf("插件「%s」版本较新但实例包未携带插件文件，保留目标机现有版本", extension.Name))
					continue
				}
			}
		}

		stagingDir := filepath.Join(stagingRoot, "extensions", extensionID)
		hasArtifact, err := extractProfilePackageExtensionArtifact(files, extensionID, stagingDir)
		if err != nil {
			return nil, err
		}
		if !hasArtifact || !backupExtensionManifestExists(stagingDir) {
			extension.InstallDir = ""
			extension.PackagePath = ""
			extension.PackageHash = ""
			if looksLikeLocalFilesystemPath(extension.SourceURL) {
				extension.SourceURL = ""
			}
			appendProfilePackageWarning(warnings, fmt.Sprintf("插件「%s」没有可迁移文件，已移除旧机器路径；请在新机器重新安装", extension.Name))
			continue
		}

		finalDir := filepath.Join(a.browserMgr.ResolveRelativePath(filepath.Join("data", "extensions")), extensionID)
		extension.InstallDir = finalDir
		extension.PackagePath = ""
		extension.PackageHash = ""
		if looksLikeLocalFilesystemPath(extension.SourceURL) {
			extension.SourceURL = ""
		}
		prepared = append(prepared, preparedProfilePackageExtensionArtifact{
			ExtensionID: extensionID,
			StagingDir:  stagingDir,
			FinalDir:    finalDir,
		})
	}
	return prepared, nil
}

func (a *App) profilePackageExistingExtension(extensionID string) (*browser.Extension, bool, error) {
	if a == nil || a.browserMgr == nil || a.browserMgr.ExtensionDAO == nil {
		return nil, false, nil
	}
	extension, err := a.browserMgr.ExtensionDAO.Get(extensionID)
	if err == nil {
		installDir := strings.TrimSpace(extension.InstallDir)
		if installDir != "" && !filepath.IsAbs(installDir) {
			installDir = a.browserMgr.ResolveRelativePath(installDir)
		}
		if installDir != "" && backupExtensionManifestExists(installDir) {
			return &extension, true, nil
		}
		packagePath := strings.TrimSpace(extension.PackagePath)
		if packagePath != "" && !filepath.IsAbs(packagePath) {
			packagePath = a.browserMgr.ResolveRelativePath(packagePath)
		}
		if packagePath != "" && backupValidCRXFile(packagePath) {
			return &extension, true, nil
		}
		return &extension, false, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("检查现有插件失败(%s): %w", extensionID, err)
}

func decideProfilePackageExtensionImport(existing, incoming browser.Extension) (profilePackageExtensionImportDecision, string) {
	existingHash := strings.TrimSpace(existing.PackageHash)
	incomingHash := strings.TrimSpace(incoming.PackageHash)
	if existingHash != "" && incomingHash != "" && strings.EqualFold(existingHash, incomingHash) {
		return profilePackageExtensionReuse, "（包哈希一致）"
	}

	comparison, comparable := compareExtensionVersions(incoming.Version, existing.Version)
	if comparable {
		if comparison > 0 {
			return profilePackageExtensionUpdate, fmt.Sprintf("（%s → %s）", existing.Version, incoming.Version)
		}
		if comparison < 0 {
			return profilePackageExtensionReuse, fmt.Sprintf("（目标机版本 %s 更新）", existing.Version)
		}
	}

	if existingHash != "" && incomingHash != "" && !strings.EqualFold(existingHash, incomingHash) {
		return profilePackageExtensionConflict, "（相同版本或不可比较版本，但包哈希不同）"
	}
	existingSource := normalizeExtensionSourceIdentity(existing.SourceURL)
	incomingSource := normalizeExtensionSourceIdentity(incoming.SourceURL)
	if existingSource != "" && incomingSource != "" && existingSource != incomingSource {
		return profilePackageExtensionConflict, "（来源不同）"
	}
	return profilePackageExtensionReuse, ""
}

func compareExtensionVersions(left, right string) (int, bool) {
	parse := func(value string) ([]int, bool) {
		parts := strings.Split(strings.TrimSpace(value), ".")
		if len(parts) == 0 || len(parts) > 4 {
			return nil, false
		}
		result := make([]int, 4)
		for index, part := range parts {
			if part == "" {
				return nil, false
			}
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 {
				return nil, false
			}
			result[index] = n
		}
		return result, true
	}
	leftParts, leftOK := parse(left)
	rightParts, rightOK := parse(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	for index := range leftParts {
		if leftParts[index] > rightParts[index] {
			return 1, true
		}
		if leftParts[index] < rightParts[index] {
			return -1, true
		}
	}
	return 0, true
}

func normalizeExtensionSourceIdentity(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || looksLikeLocalFilesystemPath(value) {
		return ""
	}
	return strings.ToLower(strings.TrimRight(value, "/"))
}

func extractProfilePackageExtensionArtifact(files []*zip.File, extensionID string, destDir string) (bool, error) {
	prefix := filepath.ToSlash(filepath.Join(profilePackageExtensionRoot, extensionID)) + "/"
	hasArtifact := false
	for _, file := range files {
		name := filepath.ToSlash(file.Name)
		if !strings.HasPrefix(name, prefix) || file.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}
		if err := extractProfilePackageFile(file, destDir, rel); err != nil {
			return false, fmt.Errorf("恢复插件文件失败(%s): %w", extensionID, err)
		}
		hasArtifact = true
	}
	return hasArtifact, nil
}

func looksLikeLocalFilesystemPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) || strings.HasPrefix(value, `\\`) || strings.HasPrefix(value, `//`) {
		return true
	}
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func applyPreparedProfilePackageExtensionArtifacts(prepared []preparedProfilePackageExtensionArtifact, swaps *[]profilePackageDirectorySwap) error {
	for _, artifact := range prepared {
		swap, err := replaceProfileUserDataDirWithBackup(artifact.StagingDir, artifact.FinalDir)
		if err != nil {
			return fmt.Errorf("提交插件迁移目录失败(%s): %w", artifact.ExtensionID, err)
		}
		*swaps = append(*swaps, swap)
	}
	return nil
}

func profilePackageExtensionArtifactPresent(files []*zip.File, extensionID string) bool {
	prefix := filepath.ToSlash(filepath.Join(profilePackageExtensionRoot, extensionID)) + "/"
	for _, file := range files {
		if strings.HasPrefix(filepath.ToSlash(file.Name), prefix) && !file.FileInfo().IsDir() {
			return true
		}
	}
	return false
}

func profilePackageExtensionArtifactDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
