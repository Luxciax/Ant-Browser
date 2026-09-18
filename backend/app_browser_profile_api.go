package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"strings"
	"time"
)

type BrowserProfile = browser.Profile
type BrowserProfileInput = browser.ProfileInput
type BrowserTab = browser.Tab
type BrowserSettings = browser.Settings
type BrowserProxy = browser.Proxy
type BrowserCore = browser.Core
type BrowserCoreInput = browser.CoreInput
type BrowserCoreValidateResult = browser.CoreValidateResult
type BrowserCoreExtendedInfo = browser.CoreExtendedInfo
type BrowserProfileCopyOptions = browser.ProfileCopyOptions

func (a *App) snapshotManagedBrowserProfile(profile *BrowserProfile) *BrowserProfile {
	if profile == nil {
		return nil
	}
	if a == nil || a.browserMgr == nil {
		return copyBrowserProfileSnapshot(profile)
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()
	return copyBrowserProfileSnapshot(profile)
}

// BrowserProfileList 获取所有实例列表
func (a *App) BrowserProfileList() []BrowserProfile {
	if a == nil || a.browserMgr == nil {
		return []BrowserProfile{}
	}
	return a.browserMgr.List()
}

// BrowserProfileListByTag 按标签筛选实例列表
func (a *App) BrowserProfileListByTag(tag string) []BrowserProfile {
	if a == nil || a.browserMgr == nil {
		return []BrowserProfile{}
	}
	return a.browserMgr.ListByTag(tag)
}

// BrowserGetAllTags 获取所有已使用的标签
func (a *App) BrowserGetAllTags() []string {
	if a == nil || a.browserMgr == nil {
		return []string{}
	}
	return a.browserMgr.GetAllTags()
}

// BrowserProfileSetKeywords 设置实例关键字
func (a *App) BrowserProfileSetKeywords(profileId string, keywords []string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	profile, err := a.browserMgr.SetKeywords(profileId, keywords)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

func (a *App) BrowserProfileCreate(input BrowserProfileInput) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	profile, err := a.browserMgr.Create(input)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

func (a *App) BrowserProfileUpdate(profileId string, input BrowserProfileInput) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	profile, err := a.browserMgr.Update(profileId, input)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

func (a *App) BrowserProfileDelete(profileId string) error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	return a.browserMgr.Delete(profileId)
}

// BrowserProfileTrashList 获取回收站实例列表
func (a *App) BrowserProfileTrashList() []BrowserProfile { return a.browserMgr.ListDeleted() }

// BrowserProfileRestore 从回收站恢复实例
func (a *App) BrowserProfileRestore(profileId string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	profile, err := a.browserMgr.Restore(profileId)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

// BrowserProfilePermanentlyDelete 从回收站彻底删除实例
func (a *App) BrowserProfilePermanentlyDelete(profileId string) error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	unlockRuntimeOp := a.lockProfileRuntimeOperation(profileId)
	defer unlockRuntimeOp()
	return a.browserMgr.PermanentlyDelete(profileId)
}

// BrowserProfileTrashCleanup 清理超过保留期的回收站实例
func (a *App) BrowserProfileTrashCleanup() error {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	return a.browserMgr.CleanupExpiredTrash()
}

// BrowserProfileCopy 复制实例配置（除指纹参数外全部复制）
func (a *App) BrowserProfileCopy(profileId string, newName string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	profile, err := a.browserMgr.Copy(profileId, newName)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

// BrowserProfileCopyWithMode 按模式复制实例配置。
func (a *App) BrowserProfileCopyWithMode(profileId string, newName string, mode string) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	profile, err := a.browserMgr.CopyWithMode(profileId, newName, mode)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

// BrowserProfileCopyWithOptions 按结构化选项复制实例配置。
func (a *App) BrowserProfileCopyWithOptions(profileId string, newName string, options BrowserProfileCopyOptions) (*BrowserProfile, error) {
	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()
	profile, err := a.browserMgr.CopyWithOptions(profileId, newName, options)
	if err != nil {
		return nil, err
	}
	return a.snapshotManagedBrowserProfile(profile), nil
}

// migrateToSQLite 一次性迁移：若 SQLite 表为空则从旧文件导入数据，或初始化默认数据
// 迁移顺序：cores → proxies → profiles → bookmarks
func (a *App) migrateToSQLite() {
	log := logger.New("Migration")

	if cores, err := a.browserMgr.CoreDAO.List(); err != nil {
		log.Error("读取内核迁移目标失败", logger.F("error", err))
	} else if len(cores) == 0 {
		if len(a.config.Browser.Cores) > 0 {
			if bulk, ok := a.browserMgr.CoreDAO.(interface{ UpsertAll([]browser.Core) error }); ok {
				if err := bulk.UpsertAll(a.config.Browser.Cores); err != nil {
					log.Error("内核迁移失败", logger.F("error", err))
				} else {
					log.Info("内核数据已迁移", logger.F("count", len(a.config.Browser.Cores)))
				}
			} else {
				log.Error("内核迁移失败", logger.F("error", "当前 CoreDAO 不支持原子批量迁移"))
			}
		} else {
			log.Info("内核表为空，将通过自动检测初始化")
		}
	}

	if proxies, err := a.browserMgr.ProxyDAO.List(); err != nil {
		log.Error("读取代理迁移目标失败", logger.F("error", err))
	} else if len(proxies) == 0 {
		var srcProxies []browser.Proxy
		if loaded, err := config.LoadProxies(a.resolveAppPath("proxies.yaml")); err == nil && len(loaded) > 0 {
			srcProxies = loaded
		} else if len(a.config.Browser.Proxies) > 0 {
			srcProxies = a.config.Browser.Proxies
		} else {
			srcProxies = []browser.Proxy{
				{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
			}
			log.Info("代理表为空，初始化默认代理")
		}
		if err := a.browserMgr.ProxyDAO.ReplaceAll(srcProxies); err != nil {
			log.Error("代理迁移失败", logger.F("error", err))
		} else if len(srcProxies) > 0 {
			log.Info("代理数据已初始化", logger.F("count", len(srcProxies)))
		}
	}

	if profiles, err := a.browserMgr.ProfileDAO.List(); err != nil {
		log.Error("读取实例迁移目标失败", logger.F("error", err))
	} else if len(profiles) == 0 {
		if len(a.config.Browser.Profiles) > 0 {
			migratedProfiles := make([]*browser.Profile, 0, len(a.config.Browser.Profiles))
			for _, pc := range a.config.Browser.Profiles {
				coreId := strings.TrimSpace(pc.CoreId)
				if strings.EqualFold(coreId, "default") {
					coreId = ""
				}
				p := &browser.Profile{
					ProfileId:          pc.ProfileId,
					ProfileName:        pc.ProfileName,
					UserDataDir:        pc.UserDataDir,
					CoreId:             coreId,
					RestoreLastSession: pc.RestoreLastSession,
					FingerprintArgs:    pc.FingerprintArgs,
					ProxyId:            pc.ProxyId,
					ProxyConfig:        pc.ProxyConfig,
					ProxyBindSourceID:  pc.ProxyBindSourceID,
					ProxyBindSourceURL: pc.ProxyBindSourceURL,
					ProxyBindName:      pc.ProxyBindName,
					ProxyBindUpdatedAt: pc.ProxyBindUpdatedAt,
					MemoryLimitMB:      pc.MemoryLimitMB,
					LaunchArgs:         pc.LaunchArgs,
					Tags:               pc.Tags,
					Keywords:           pc.Keywords,
					RuntimeState:       browser.RuntimeStopped,
					CreatedAt:          pc.CreatedAt,
					UpdatedAt:          pc.UpdatedAt,
				}
				migratedProfiles = append(migratedProfiles, p)
			}
			if bulk, ok := a.browserMgr.ProfileDAO.(interface{ UpsertAll([]*browser.Profile) error }); ok {
				if err := bulk.UpsertAll(migratedProfiles); err != nil {
					log.Error("实例迁移失败", logger.F("error", err))
				} else {
					log.Info("实例数据已迁移", logger.F("count", len(migratedProfiles)))
				}
			} else {
				log.Error("实例迁移失败", logger.F("error", "当前 ProfileDAO 不支持原子批量迁移"))
			}
		} else {
			log.Info("实例表为空，自动创建默认实例")
			defaultProfile := &browser.Profile{
				ProfileId:       generateUUID(),
				ProfileName:     "默认实例",
				UserDataDir:     "default",
				CoreId:          "",
				FingerprintArgs: a.config.Browser.DefaultFingerprintArgs,
				LaunchArgs:      a.config.Browser.DefaultLaunchArgs,
				Tags:            []string{"默认"},
				ProxyId:         "__direct__",
				ProxyConfig:     "direct://",
				RuntimeState:    browser.RuntimeStopped,
				CreatedAt:       time.Now().Format(time.RFC3339),
				UpdatedAt:       time.Now().Format(time.RFC3339),
			}
			if err := a.browserMgr.ProfileDAO.Upsert(defaultProfile); err != nil {
				log.Error("自动创建默认实例失败", logger.F("error", err))
			}
		}
	}

	if bookmarks, err := a.browserMgr.BookmarkDAO.List(); err == nil && len(bookmarks) == 0 {
		src := a.config.Browser.DefaultBookmarks
		if len(src) == 0 {
			src = []config.BrowserBookmark{
				{Name: "Google", URL: "https://www.google.com/"},
				{Name: "Gmail", URL: "https://mail.google.com/"},
				{Name: "Claude", URL: "https://claude.ai/"},
				{Name: "ChatGPT", URL: "https://chatgpt.com/"},
				{Name: "YouTube", URL: "https://www.youtube.com/"},
			}
		}
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(src); err != nil {
			log.Error("书签迁移失败", logger.F("error", err))
		} else {
			log.Info("书签数据已迁移", logger.F("count", len(src)))
		}
	}
}
