package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/secretstore"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	backupLocalConfigFileName = "backup.local.yaml"
	backupLocalConfigDirName  = "Ant Browser"
	backupSecretStoreFileName  = "secrets.v1.json"
)

func defaultBackupLocalConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, backupLocalConfigDirName, backupLocalConfigFileName)
}

func defaultSecretStorePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, backupLocalConfigDirName, backupSecretStoreFileName)
}

func backupLocalSecretStore(path string, preferred *secretstore.Store) (*secretstore.Store, error) {
	if preferred != nil {
		return preferred, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("本地备份配置路径为空")
	}
	return secretstore.New(filepath.Join(filepath.Dir(filepath.Clean(path)), backupSecretStoreFileName))
}

func (a *App) backupLocalConfigPath() string {
	if a == nil {
		return ""
	}
	if override := strings.TrimSpace(a.backupLocalConfigPathOverride); override != "" {
		return filepath.Clean(override)
	}
	return a.resolveAppPath(backupLocalConfigFileName)
}

func (a *App) legacyBackupLocalConfigPath() string {
	if a == nil {
		return ""
	}
	return a.resolveAppPath(backupLocalConfigFileName)
}

func (a *App) activateStableBackupLocalConfig() {
	if a == nil {
		return
	}
	if path := defaultBackupLocalConfigPath(); path != "" {
		a.backupLocalConfigPathOverride = path
	}
}

func (a *App) loadBackupLocalConfig(base config.BackupConfig) (config.BackupConfig, bool, string, error) {
	path := a.backupLocalConfigPath()
	store, storeErr := backupLocalSecretStore(path, a.secretStore)
	if storeErr != nil { return base, false, path, storeErr }
	settings, exists, err := loadBackupLocalConfigWithSecretStore(path, base, store)
	if err != nil || exists {
		return settings, exists, path, err
	}

	legacyPath := a.legacyBackupLocalConfigPath()
	if legacyPath == path {
		return settings, false, "", nil
	}
	legacyStore, legacyStoreErr := backupLocalSecretStore(legacyPath, a.secretStore)
	if legacyStoreErr != nil { return base, false, legacyPath, legacyStoreErr }
	legacySettings, legacyExists, legacyErr := loadBackupLocalConfigWithSecretStore(legacyPath, base, legacyStore)
	return legacySettings, legacyExists, legacyPath, legacyErr
}

func (a *App) backupLocalConfigPaths() []string {
	if a == nil {
		return nil
	}
	paths := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	for _, path := range []string{a.backupLocalConfigPath(), a.legacyBackupLocalConfigPath()} {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (a *App) removeBackupLocalConfigs() error {
	for _, path := range a.backupLocalConfigPaths() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除本地备份配置失败: %w", err)
		}
	}
	return nil
}

type backupLocalConfigFile struct {
	Backup backupLocalSecrets `yaml:"backup"`
}

type backupLocalSecrets struct {
	Channels backupLocalChannelsSecrets `yaml:"channels"`
}

type backupLocalChannelsSecrets struct {
	OpenList backupLocalOpenListSecrets `yaml:"openlist"`
	S3       backupLocalS3Config        `yaml:"s3,omitempty"`
}

type backupLocalOpenListSecrets struct {
	TokenRef string `yaml:"token_ref,omitempty"`
}

type backupLocalOpenListConfigRead struct {
	BaseURL             string `yaml:"base_url,omitempty"`
	RemotePath          string `yaml:"remote_path,omitempty"`
	Token                string `yaml:"token,omitempty"`
	TokenRef             string `yaml:"token_ref,omitempty"`
	UploadRateLimitMBps int    `yaml:"upload_rate_limit_mbps,omitempty"`
}

type backupLocalS3Config struct {
	Endpoint           string `yaml:"endpoint,omitempty"`
	Region             string `yaml:"region,omitempty"`
	Bucket             string `yaml:"bucket,omitempty"`
	Prefix             string `yaml:"prefix,omitempty"`
	AccessKeyIDRef     string `yaml:"access_key_id_ref,omitempty"`
	SecretAccessKeyRef string `yaml:"secret_access_key_ref,omitempty"`
	SessionTokenRef    string `yaml:"session_token_ref,omitempty"`
	ForcePathStyle     bool   `yaml:"force_path_style,omitempty"`
}

type backupLocalS3ConfigRead struct {
	Endpoint           string `yaml:"endpoint,omitempty"`
	Region             string `yaml:"region,omitempty"`
	Bucket             string `yaml:"bucket,omitempty"`
	Prefix             string `yaml:"prefix,omitempty"`
	AccessKeyID        string `yaml:"access_key_id,omitempty"`
	SecretAccessKey    string `yaml:"secret_access_key,omitempty"`
	SessionToken       string `yaml:"session_token,omitempty"`
	AccessKeyIDRef     string `yaml:"access_key_id_ref,omitempty"`
	SecretAccessKeyRef string `yaml:"secret_access_key_ref,omitempty"`
	SessionTokenRef    string `yaml:"session_token_ref,omitempty"`
	ForcePathStyle     *bool  `yaml:"force_path_style,omitempty"`
}

type backupLocalConfigReadFile struct {
	Backup backupLocalConfigReadBackup `yaml:"backup"`
}

type backupLocalConfigReadBackup struct {
	Channels backupLocalConfigReadChannels `yaml:"channels"`
	OpenList config.OpenListChannelConfig  `yaml:"openlist"`
	Schedule config.BackupScheduleConfig   `yaml:"schedule"`
}

type backupLocalConfigReadChannels struct {
	OpenList backupLocalOpenListConfigRead `yaml:"openlist"`
	S3       backupLocalS3ConfigRead       `yaml:"s3"`
}

func defaultBackupConfig() config.BackupConfig {
	return config.DefaultConfig().Backup
}

func normalizeBackupSettings(value config.BackupConfig) config.BackupConfig {
	defaults := defaultBackupConfig()
	value.LocalDirectory = strings.TrimSpace(value.LocalDirectory)
	value.Channels.OpenList.BaseURL = strings.TrimSpace(value.Channels.OpenList.BaseURL)
	value.Channels.OpenList.RemotePath = strings.TrimSpace(value.Channels.OpenList.RemotePath)
	if value.Channels.OpenList.RemotePath == "" {
		value.Channels.OpenList.RemotePath = defaults.Channels.OpenList.RemotePath
	}
	value.Channels.OpenList.Token = strings.TrimSpace(value.Channels.OpenList.Token)
	value.Channels.S3.Endpoint = strings.TrimSpace(value.Channels.S3.Endpoint)
	value.Channels.S3.Region = strings.TrimSpace(value.Channels.S3.Region)
	if value.Channels.S3.Region == "" {
		value.Channels.S3.Region = defaults.Channels.S3.Region
	}
	value.Channels.S3.Bucket = strings.TrimSpace(value.Channels.S3.Bucket)
	value.Channels.S3.Prefix = strings.TrimSpace(value.Channels.S3.Prefix)
	value.Channels.S3.AccessKeyID = strings.TrimSpace(value.Channels.S3.AccessKeyID)
	value.Channels.S3.SecretAccessKey = strings.TrimSpace(value.Channels.S3.SecretAccessKey)
	value.Channels.S3.SessionToken = strings.TrimSpace(value.Channels.S3.SessionToken)
	value.Schedule.DailyTime = strings.TrimSpace(value.Schedule.DailyTime)
	if value.Schedule.DailyTime == "" {
		value.Schedule.DailyTime = defaults.Schedule.DailyTime
	}
	value.Schedule.RecentBackupTimes = normalizeBackupScheduleTimes(value.Schedule.RecentBackupTimes)
	return value
}

func backupConfigsEqual(left, right config.BackupConfig) bool {
	left = normalizeBackupSettings(left)
	right = normalizeBackupSettings(right)
	return left.LocalDirectory == right.LocalDirectory &&
		left.Channels.OpenList.BaseURL == right.Channels.OpenList.BaseURL &&
		left.Channels.OpenList.RemotePath == right.Channels.OpenList.RemotePath &&
		left.Channels.OpenList.Token == right.Channels.OpenList.Token &&
		left.Channels.OpenList.UploadRateLimitMBps == right.Channels.OpenList.UploadRateLimitMBps &&
		left.Channels.S3.Endpoint == right.Channels.S3.Endpoint &&
		left.Channels.S3.Region == right.Channels.S3.Region &&
		left.Channels.S3.Bucket == right.Channels.S3.Bucket &&
		left.Channels.S3.Prefix == right.Channels.S3.Prefix &&
		left.Channels.S3.AccessKeyID == right.Channels.S3.AccessKeyID &&
		left.Channels.S3.SecretAccessKey == right.Channels.S3.SecretAccessKey &&
		left.Channels.S3.SessionToken == right.Channels.S3.SessionToken &&
		left.Channels.S3.ForcePathStyle == right.Channels.S3.ForcePathStyle &&
		left.Schedule.Enabled == right.Schedule.Enabled &&
		left.Schedule.DailyTime == right.Schedule.DailyTime &&
		backupScheduleTimesEqual(left.Schedule.RecentBackupTimes, right.Schedule.RecentBackupTimes)
}

func backupScheduleTimesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func backupHasLocalSecrets(value config.BackupConfig) bool {
	return strings.TrimSpace(value.Channels.OpenList.Token) != "" ||
		strings.TrimSpace(value.Channels.S3.AccessKeyID) != "" ||
		strings.TrimSpace(value.Channels.S3.SecretAccessKey) != "" ||
		strings.TrimSpace(value.Channels.S3.SessionToken) != ""
}

func backupHasS3Config(value config.BackupConfig) bool {
	s3 := value.Channels.S3
	defaultRegion := defaultBackupConfig().Channels.S3.Region
	return strings.TrimSpace(s3.Endpoint) != "" ||
		(strings.TrimSpace(s3.Region) != "" && strings.TrimSpace(s3.Region) != strings.TrimSpace(defaultRegion)) ||
		strings.TrimSpace(s3.Bucket) != "" ||
		strings.TrimSpace(s3.Prefix) != "" ||
		strings.TrimSpace(s3.AccessKeyID) != "" ||
		strings.TrimSpace(s3.SecretAccessKey) != "" ||
		strings.TrimSpace(s3.SessionToken) != "" ||
		s3.ForcePathStyle
}

func backupHasLocalConfig(value config.BackupConfig) bool {
	return backupHasLocalSecrets(value) || backupHasS3Config(value)
}

func loadBackupLocalConfig(path string, base config.BackupConfig) (config.BackupConfig, bool, error) {
	store, err := backupLocalSecretStore(path, nil)
	if err != nil { return normalizeBackupSettings(base), false, err }
	return loadBackupLocalConfigWithSecretStore(path, base, store)
}

func loadBackupLocalConfigWithSecretStore(path string, base config.BackupConfig, store *secretstore.Store) (config.BackupConfig, bool, error) {
	settings := normalizeBackupSettings(base)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, false, nil
		}
		return settings, true, fmt.Errorf("读取本地备份配置失败: %w", err)
	}

	var stored backupLocalConfigReadFile
	if err := yaml.Unmarshal(data, &stored); err != nil {
		return settings, true, fmt.Errorf("解析本地备份配置失败: %w", err)
	}

	localOpenList := stored.Backup.Channels.OpenList
	legacyOpenList := stored.Backup.OpenList
	if strings.TrimSpace(localOpenList.BaseURL) == "" {
		localOpenList.BaseURL = legacyOpenList.BaseURL
	}
	if strings.TrimSpace(localOpenList.RemotePath) == "" {
		localOpenList.RemotePath = legacyOpenList.RemotePath
	}
	resolvedOpenListToken := strings.TrimSpace(localOpenList.Token)
	if strings.TrimSpace(localOpenList.TokenRef) != "" {
		if store == nil {
			return settings, true, fmt.Errorf("OpenList token uses SecretRef but SecretStore is unavailable")
		}
		value, resolveErr := store.Get(localOpenList.TokenRef)
		if resolveErr != nil {
			return settings, true, fmt.Errorf("读取 OpenList SecretRef 失败: %w", resolveErr)
		}
		resolvedOpenListToken = value
	}
	if resolvedOpenListToken == "" {
		resolvedOpenListToken = strings.TrimSpace(legacyOpenList.Token)
	}
	localS3 := stored.Backup.Channels.S3
	resolveSecret := func(ref, legacyValue, label string) (string, error) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return strings.TrimSpace(legacyValue), nil
		}
		if store == nil {
			return "", fmt.Errorf("%s uses SecretRef but SecretStore is unavailable", label)
		}
		value, resolveErr := store.Get(ref)
		if resolveErr != nil {
			return "", fmt.Errorf("读取 %s SecretRef 失败: %w", label, resolveErr)
		}
		return strings.TrimSpace(value), nil
	}
	accessKeyID, secretErr := resolveSecret(localS3.AccessKeyIDRef, localS3.AccessKeyID, "S3 access key")
	if secretErr != nil { return settings, true, secretErr }
	secretAccessKey, secretErr := resolveSecret(localS3.SecretAccessKeyRef, localS3.SecretAccessKey, "S3 secret access key")
	if secretErr != nil { return settings, true, secretErr }
	sessionToken, secretErr := resolveSecret(localS3.SessionTokenRef, localS3.SessionToken, "S3 session token")
	if secretErr != nil { return settings, true, secretErr }

	if strings.TrimSpace(base.Channels.OpenList.BaseURL) == "" {
		settings.Channels.OpenList.BaseURL = strings.TrimSpace(localOpenList.BaseURL)
	}
	defaults := defaultBackupConfig()
	if strings.TrimSpace(base.Channels.OpenList.RemotePath) == "" ||
		(strings.TrimSpace(base.Channels.OpenList.RemotePath) == defaults.Channels.OpenList.RemotePath && strings.TrimSpace(localOpenList.RemotePath) != "" && strings.TrimSpace(localOpenList.RemotePath) != defaults.Channels.OpenList.RemotePath) {
		settings.Channels.OpenList.RemotePath = strings.TrimSpace(localOpenList.RemotePath)
	}
	if resolvedOpenListToken != "" {
		settings.Channels.OpenList.Token = strings.TrimSpace(resolvedOpenListToken)
	}
	if value := strings.TrimSpace(localS3.Endpoint); value != "" {
		settings.Channels.S3.Endpoint = value
	}
	if value := strings.TrimSpace(localS3.Region); value != "" {
		settings.Channels.S3.Region = value
	}
	if value := strings.TrimSpace(localS3.Bucket); value != "" {
		settings.Channels.S3.Bucket = value
	}
	if value := strings.TrimSpace(localS3.Prefix); value != "" {
		settings.Channels.S3.Prefix = value
	}
	if accessKeyID != "" {
		settings.Channels.S3.AccessKeyID = accessKeyID
	}
	if secretAccessKey != "" {
		settings.Channels.S3.SecretAccessKey = secretAccessKey
	}
	if sessionToken != "" {
		settings.Channels.S3.SessionToken = sessionToken
	}
	if localS3.ForcePathStyle != nil {
		settings.Channels.S3.ForcePathStyle = *localS3.ForcePathStyle
	}
	if (strings.TrimSpace(base.Schedule.DailyTime) == "" && !base.Schedule.Enabled) ||
		(!base.Schedule.Enabled && strings.TrimSpace(base.Schedule.DailyTime) == defaults.Schedule.DailyTime && strings.TrimSpace(stored.Backup.Schedule.DailyTime) != "" && strings.TrimSpace(stored.Backup.Schedule.DailyTime) != defaults.Schedule.DailyTime) {
		if strings.TrimSpace(stored.Backup.Schedule.DailyTime) != "" || stored.Backup.Schedule.Enabled {
			settings.Schedule = stored.Backup.Schedule
		}
	}

	return normalizeBackupSettings(settings), true, nil
}

func saveBackupLocalConfig(path string, value config.BackupConfig) error {
	store, err := backupLocalSecretStore(path, nil)
	if err != nil { return err }
	return saveBackupLocalConfigWithSecretStore(path, value, store)
}

func (a *App) saveBackupLocalConfig(path string, value config.BackupConfig) error {
	store, err := backupLocalSecretStore(path, a.secretStore)
	if err != nil { return err }
	return saveBackupLocalConfigWithSecretStore(path, value, store)
}

func loadExistingBackupSecretRefs(path string) backupLocalConfigReadFile {
	var stored backupLocalConfigReadFile
	data, err := os.ReadFile(path)
	if err == nil {
		_ = yaml.Unmarshal(data, &stored)
	}
	return stored
}

func stageBackupSecret(store *secretstore.Store, label, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if store == nil {
		return "", fmt.Errorf("%s cannot be stored because SecretStore is unavailable", label)
	}
	return store.Set("", label, value)
}

func cleanupStagedBackupSecrets(store *secretstore.Store, refs ...string) {
	if store == nil { return }
	for _, ref := range refs {
		if strings.TrimSpace(ref) != "" {
			_ = store.Delete(ref)
		}
	}
}

func cleanupReplacedBackupSecrets(store *secretstore.Store, oldRefs, newRefs []string) {
	if store == nil { return }
	for index, oldRef := range oldRefs {
		oldRef = strings.TrimSpace(oldRef)
		if oldRef == "" { continue }
		newRef := ""
		if index < len(newRefs) { newRef = strings.TrimSpace(newRefs[index]) }
		if oldRef != newRef {
			_ = store.Delete(oldRef)
		}
	}
}

func saveBackupLocalConfigWithSecretStore(path string, value config.BackupConfig, store *secretstore.Store) error {
	value = normalizeBackupSettings(value)
	existing := loadExistingBackupSecretRefs(path)
	tokenRef, err := stageBackupSecret(store, "backup.openlist.token", value.Channels.OpenList.Token)
	if err != nil {
		return fmt.Errorf("保护 OpenList Token 失败: %w", err)
	}
	accessKeyRef, err := stageBackupSecret(store, "backup.s3.accessKeyId", value.Channels.S3.AccessKeyID)
	if err != nil {
		cleanupStagedBackupSecrets(store, tokenRef)
		return fmt.Errorf("保护 S3 AccessKeyID 失败: %w", err)
	}
	secretKeyRef, err := stageBackupSecret(store, "backup.s3.secretAccessKey", value.Channels.S3.SecretAccessKey)
	if err != nil {
		cleanupStagedBackupSecrets(store, tokenRef, accessKeyRef)
		return fmt.Errorf("保护 S3 SecretAccessKey 失败: %w", err)
	}
	sessionTokenRef, err := stageBackupSecret(store, "backup.s3.sessionToken", value.Channels.S3.SessionToken)
	if err != nil {
		cleanupStagedBackupSecrets(store, tokenRef, accessKeyRef, secretKeyRef)
		return fmt.Errorf("保护 S3 SessionToken 失败: %w", err)
	}
	stagedRefs := []string{tokenRef, accessKeyRef, secretKeyRef, sessionTokenRef}
	oldRefs := []string{
		existing.Backup.Channels.OpenList.TokenRef,
		existing.Backup.Channels.S3.AccessKeyIDRef,
		existing.Backup.Channels.S3.SecretAccessKeyRef,
		existing.Backup.Channels.S3.SessionTokenRef,
	}

	hasS3 := backupHasS3Config(value)
	if tokenRef == "" && !hasS3 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			cleanupStagedBackupSecrets(store, stagedRefs...)
			return fmt.Errorf("删除本地备份配置失败: %w", err)
		}
		cleanupReplacedBackupSecrets(store, oldRefs, stagedRefs)
		return nil
	}
	s3Config := backupLocalS3Config{}
	if hasS3 {
		s3Config = backupLocalS3Config{
			Endpoint: strings.TrimSpace(value.Channels.S3.Endpoint),
			Region: strings.TrimSpace(value.Channels.S3.Region),
			Bucket: strings.TrimSpace(value.Channels.S3.Bucket),
			Prefix: strings.TrimSpace(value.Channels.S3.Prefix),
			AccessKeyIDRef: accessKeyRef,
			SecretAccessKeyRef: secretKeyRef,
			SessionTokenRef: sessionTokenRef,
			ForcePathStyle: value.Channels.S3.ForcePathStyle,
		}
	}
	stored := backupLocalConfigFile{
		Backup: backupLocalSecrets{
			Channels: backupLocalChannelsSecrets{
				OpenList: backupLocalOpenListSecrets{TokenRef: tokenRef},
				S3: s3Config,
			},
		},
	}
	data, err := yaml.Marshal(stored)
	if err != nil {
		cleanupStagedBackupSecrets(store, stagedRefs...)
		return fmt.Errorf("序列化本地备份配置失败: %w", err)
	}
	if err := fsutil.AtomicWriteFile(path, data, 0o600); err != nil {
		cleanupStagedBackupSecrets(store, stagedRefs...)
		return fmt.Errorf("写入本地备份配置失败: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("设置本地备份配置权限失败: %w", err)
	}
	cleanupReplacedBackupSecrets(store, oldRefs, stagedRefs)
	return nil
}

func (a *App) prepareBackupLocalConfig() error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}

	path := a.backupLocalConfigPath()
	base := normalizeBackupSettings(a.config.Backup)
	settings, _, sourcePath, err := a.loadBackupLocalConfig(a.config.Backup)
	if err != nil {
		if !backupHasLocalSecrets(base) {
			return err
		}
		settings = base
	}
	if backupHasLocalConfig(settings) {
		if err := a.saveBackupLocalConfig(path, settings); err != nil {
			return fmt.Errorf("保存本地备份配置失败: %w", err)
		}
		if sourcePath != "" && sourcePath != path {
			if err := os.Remove(sourcePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("删除旧本地备份配置失败: %w", err)
			}
		}
	}

	settings = normalizeBackupSettings(settings)
	if backupConfigsEqual(a.config.Backup, settings) && !backupHasLocalConfig(base) {
		return nil
	}
	previous := a.config.Backup
	a.config.Backup = settings
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		a.config.Backup = previous
		return fmt.Errorf("保存备份配置失败: %w", err)
	}
	return nil
}
