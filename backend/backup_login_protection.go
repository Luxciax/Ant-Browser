package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const backupLoginProtectionFile = "backup.login.bin"

type backupLoginProtection struct {
	Version   int    `json:"version"`
	Enabled   bool   `json:"enabled"`
	Protected []byte `json:"protected"`
}

func (a *App) loginProtectionPath() string {
	return filepath.Join(filepath.Dir(a.backupLocalConfigPath()), backupLoginProtectionFile)
}

func (a *App) readBackupLoginProtection() (backupLoginProtection, error) {
	data, err := os.ReadFile(a.loginProtectionPath())
	if os.IsNotExist(err) {
		return backupLoginProtection{Version: 1}, nil
	}
	if err != nil {
		return backupLoginProtection{}, fmt.Errorf("读取备份登录态保护配置失败")
	}
	var settings backupLoginProtection
	if len(data) > 64<<10 || json.Unmarshal(data, &settings) != nil || settings.Version != 1 {
		return settings, fmt.Errorf("备份登录态保护配置已损坏，请重新设置；未降级为普通备份")
	}
	return settings, nil
}

func (a *App) BackupLoginProtectionSettings() (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	settings, err := a.readBackupLoginProtection()
	if err != nil {
		return map[string]any{"enabled": true, "configured": false, "supported": runtime.GOOS == "windows", "needsRepair": true}, nil
	}
	return map[string]any{"enabled": settings.Enabled, "configured": len(settings.Protected) > 0, "supported": runtime.GOOS == "windows"}, nil
}

func (a *App) BackupSaveLoginProtection(enabled bool, password string) (map[string]any, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	settings, err := a.readBackupLoginProtection()
	// An explicitly supplied new password can repair a corrupt local record.
	if err != nil && strings.TrimSpace(password) == "" && enabled {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		return nil, errPortableLoginUnsupported
	}
	settings.Version = 1
	password = strings.TrimSpace(password)
	if password != "" {
		if len(password) < 8 {
			return nil, fmt.Errorf("迁移密码至少需要 8 个字符")
		}
		data := []byte(password)
		defer zeroSensitiveBytes(data)
		settings.Protected, err = protectProfileOSCryptKey(data)
		if err != nil {
			return nil, fmt.Errorf("迁移密码本机加密失败")
		}
	}
	if enabled {
		if len(settings.Protected) == 0 {
			return nil, fmt.Errorf("请先设置迁移密码")
		}
		plain, err := unprotectProfileOSCryptKey(settings.Protected)
		if err != nil {
			return nil, fmt.Errorf("原密码保护无法在此 Windows 用户解密，请重新输入迁移密码")
		}
		zeroSensitiveBytes(plain)
	}
	settings.Enabled = enabled
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(a.loginProtectionPath()), 0700); err != nil {
		return nil, err
	}
	if err := fsutil.AtomicWriteFile(a.loginProtectionPath(), data, 0600); err != nil {
		return nil, err
	}
	return map[string]any{"enabled": enabled, "configured": len(settings.Protected) > 0, "supported": true}, nil
}

func (a *App) automaticBackupLoginOptions() (ProfilePackageExportOptions, error) {
	settings, err := a.readBackupLoginProtection()
	if err != nil {
		return ProfilePackageExportOptions{}, err
	}
	if !settings.Enabled {
		return ProfilePackageExportOptions{}, nil
	}
	plain, err := unprotectProfileOSCryptKey(settings.Protected)
	if err != nil {
		return ProfilePackageExportOptions{}, fmt.Errorf("自动备份迁移密码无法解密，请重新设置；未降级为普通备份")
	}
	defer zeroSensitiveBytes(plain)
	if len(plain) < 8 {
		return ProfilePackageExportOptions{}, fmt.Errorf("自动备份迁移密码无效，未降级为普通备份")
	}
	return ProfilePackageExportOptions{PortableLogin: true, ProtectArchive: true, MigrationPassword: string(plain)}, nil
}
