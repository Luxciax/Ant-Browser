package backend

import (
	"ant-chrome/backend/internal/backup"
	"fmt"
	"strings"
	"time"
)

func (a *App) backupExportPackageToPath(savePath string) (map[string]interface{}, error) {
	options, err := a.automaticBackupLoginOptions()
	if err != nil {
		return nil, err
	}
	return a.backupExportPackageToPathWithOptions(savePath, options)
}

func (a *App) backupExportPackageToPathWithOptions(savePath string, options ProfilePackageExportOptions) (map[string]interface{}, error) {
	if options.ProtectArchive && (!options.PortableLogin || len(strings.TrimSpace(options.MigrationPassword)) < 8) {
		return nil, fmt.Errorf("整包加密需要有效的登录态迁移密码")
	}
	if strings.TrimSpace(savePath) == `` {
		return nil, fmt.Errorf(`backup export path is empty`)
	}
	a.backupEmitExportProgress(`starting`, 0, `checking running profiles`)
	if runningNames := a.backupRunningProfileNames(); len(runningNames) > 0 {
		message := fmt.Sprintf(`stop running profiles before export: %s`, strings.Join(runningNames, `, `))
		a.backupEmitExportProgress(`error`, 100, message)
		return nil, fmt.Errorf(`%s`, message)
	}
	a.backupEmitExportProgress(`preparing`, 8, `collecting backup scope`)
	if err := a.backupCheckpointSQLiteWAL(); err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}

	scope, err := a.backupBuildScope()
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}
	portableCount := 0
	var warnings []string
	if options.PortableLogin {
		var cleanup func()
		portableCount, warnings, cleanup, err = a.backupAddLoginState(&scope, options.MigrationPassword)
		defer cleanup()
		if err != nil {
			return nil, err
		}
	}
	manifest := backup.BuildManifest(scope, a.appName(), a.appVersion(), time.Now())
	a.backupEmitExportProgress(`preparing`, 15, `writing backup package`)

	var encryptionPassword []string
	if options.ProtectArchive {
		encryptionPassword = []string{strings.TrimSpace(options.MigrationPassword)}
	}
	includedEntries, skippedEntries, fileCount, err := backupWritePackageZip(savePath, scope, manifest, a.backupEmitExportProgressMeta, encryptionPassword...)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`backup export failed: %v`, err))
		return nil, err
	}

	return map[string]interface{}{
		`cancelled`:          false,
		`zipPath`:            savePath,
		`includedEntries`:    includedEntries,
		`skippedEntries`:     skippedEntries,
		`fileCount`:          fileCount,
		`message`:            `backup export complete`,
		`portableLoginCount`: portableCount,
		`warnings`:           warnings,
		`encryptedArchive`:   options.ProtectArchive,
	}, nil
}

func (a *App) backupExportProfilePackageToPath(savePath string, profileIDs []string) (map[string]interface{}, error) {
	return a.backupExportProfilePackageToPathWithOptions(savePath, profileIDs, ProfilePackageExportOptions{})
}

func (a *App) backupExportProfilePackageToPathWithOptions(savePath string, profileIDs []string, options ProfilePackageExportOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(savePath) == `` {
		return nil, fmt.Errorf(`backup export path is empty`)
	}
	ids := normalizeProfilePackageIDs(profileIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf(`请选择要备份的实例`)
	}

	a.backupEmitExportProgress(`starting`, 0, `正在检查选中的实例...`)
	profiles, err := a.collectProfilesForPackage(ids)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`实例备份失败: %v`, err))
		return nil, err
	}
	a.backupEmitExportProgress(`preparing`, 10, fmt.Sprintf(`正在准备 %d 个实例...`, len(profiles)))
	fileCount, portableLoginCount, warnings, err := a.writeProfilePackageWithOptions(savePath, profiles, options)
	if err != nil {
		a.backupEmitExportProgress(`error`, 100, fmt.Sprintf(`实例备份失败: %v`, err))
		return nil, err
	}
	a.backupEmitExportProgress(`done`, 100, `实例备份完成`)
	return map[string]interface{}{
		`cancelled`:          false,
		`zipPath`:            savePath,
		`profileCount`:       len(profiles),
		`profileNames`:       profilePackageProfileNames(profiles),
		`fileCount`:          fileCount,
		`packageType`:        `profile`,
		`portableLoginCount`: portableLoginCount,
		`warnings`:           warnings,
		`message`:            `实例备份完成`,
	}, nil
}
