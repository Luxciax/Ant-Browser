package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ProfileDAO 实例配置持久化接口
type ProfileDAO interface {
	List() ([]*Profile, error)
	ListDeleted() ([]*Profile, error)
	GetById(profileId string) (*Profile, error)
	Upsert(profile *Profile) error
	Delete(profileId string) error
	SoftDelete(profileId string, deletedAt string) error
	Restore(profileId string) error
	ListExpiredDeleted(expiredBefore string) ([]*Profile, error)
}

// SQLiteProfileDAO 基于 SQLite 的 ProfileDAO 实现
type SQLiteProfileDAO struct {
	db *sql.DB
}

type profileSQLExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// NewSQLiteProfileDAO 创建 SQLiteProfileDAO
func NewSQLiteProfileDAO(db *sql.DB) *SQLiteProfileDAO {
	return &SQLiteProfileDAO{db: db}
}

// List 查询所有实例配置，按创建时间升序
func (d *SQLiteProfileDAO) List() ([]*Profile, error) {
	rows, err := d.db.Query(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       COALESCE(memory_limit_mb, 0),
		       launch_args,
		       tags, keywords, group_id, created_at, updated_at,
		       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
		FROM browser_profiles WHERE COALESCE(deleted_at, '') = '' ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询实例列表失败: %w", err)
	}
	defer rows.Close()

	var list []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// ListDeleted 查询回收站实例，按删除时间倒序
func (d *SQLiteProfileDAO) ListDeleted() ([]*Profile, error) {
	rows, err := d.db.Query(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       COALESCE(memory_limit_mb, 0),
		       launch_args,
		       tags, keywords, group_id, created_at, updated_at,
		       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
		FROM browser_profiles WHERE COALESCE(deleted_at, '') != '' ORDER BY deleted_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询回收站实例失败: %w", err)
	}
	defer rows.Close()

	var list []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// GetById 根据 profileId 查询单个实例
func (d *SQLiteProfileDAO) GetById(profileId string) (*Profile, error) {
	row := d.db.QueryRow(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       COALESCE(memory_limit_mb, 0),
		       launch_args,
		       tags, keywords, group_id, created_at, updated_at,
		       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
		FROM browser_profiles WHERE profile_id = ?`, profileId)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("实例不存在: %s", profileId)
	}
	return p, err
}

// Upsert 新增或更新实例配置
func (d *SQLiteProfileDAO) Upsert(profile *Profile) error {
	createdAt, updatedAt, err := upsertProfileWithExecer(d.db, profile)
	if err != nil {
		return err
	}
	profile.CreatedAt = createdAt
	profile.UpdatedAt = updatedAt
	return nil
}

// UpsertAll 在一个事务中保存全部实例，避免批量保存时出现部分成功。
func (d *SQLiteProfileDAO) UpsertAll(profiles []*Profile) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启实例批量保存事务失败: %w", err)
	}
	defer tx.Rollback()

	type timestamps struct {
		createdAt string
		updatedAt string
	}
	committedTimestamps := make([]timestamps, len(profiles))
	for i, profile := range profiles {
		createdAt, updatedAt, err := upsertProfileWithExecer(tx, profile)
		if err != nil {
			return err
		}
		committedTimestamps[i] = timestamps{createdAt: createdAt, updatedAt: updatedAt}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交实例批量保存事务失败: %w", err)
	}
	for i, profile := range profiles {
		profile.CreatedAt = committedTimestamps[i].createdAt
		profile.UpdatedAt = committedTimestamps[i].updatedAt
	}
	return nil
}

func upsertProfileWithExecer(exec profileSQLExecer, profile *Profile) (string, string, error) {
	fingerprintArgs, _ := json.Marshal(profile.FingerprintArgs)
	launchArgs, _ := json.Marshal(profile.LaunchArgs)
	tags, _ := json.Marshal(profile.Tags)
	keywords, _ := json.Marshal(profile.Keywords)

	now := time.Now().Format(time.RFC3339)
	createdAt := profile.CreatedAt
	if createdAt == "" {
		createdAt = now
	}
	updatedAt := profile.UpdatedAt
	if updatedAt == "" {
		updatedAt = now
	}

	_, err := exec.Exec(`
		INSERT INTO browser_profiles
		  (profile_id, profile_name, user_data_dir, core_id, fingerprint_args,
		   proxy_id, proxy_config, proxy_bind_source_id, proxy_bind_source_url, proxy_bind_name, proxy_bind_updated_at,
		   memory_limit_mb, launch_args, tags, keywords, group_id, created_at, updated_at, restore_last_session, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
		  profile_name     = excluded.profile_name,
		  user_data_dir    = excluded.user_data_dir,
		  core_id          = excluded.core_id,
		  fingerprint_args = excluded.fingerprint_args,
		  proxy_id         = excluded.proxy_id,
		  proxy_config     = excluded.proxy_config,
		  proxy_bind_source_id = excluded.proxy_bind_source_id,
		  proxy_bind_source_url = excluded.proxy_bind_source_url,
		  proxy_bind_name = excluded.proxy_bind_name,
		  proxy_bind_updated_at = excluded.proxy_bind_updated_at,
		  memory_limit_mb  = excluded.memory_limit_mb,
		  launch_args      = excluded.launch_args,
		  tags             = excluded.tags,
		  keywords         = excluded.keywords,
		  group_id         = excluded.group_id,
		  restore_last_session = excluded.restore_last_session,
		  deleted_at       = excluded.deleted_at,
		  updated_at       = excluded.updated_at`,
		profile.ProfileId, profile.ProfileName, profile.UserDataDir, profile.CoreId,
		string(fingerprintArgs), profile.ProxyId, profile.ProxyConfig,
		profile.ProxyBindSourceID, profile.ProxyBindSourceURL, profile.ProxyBindName, profile.ProxyBindUpdatedAt,
		normalizeMemoryLimitMB(profile.MemoryLimitMB), string(launchArgs), string(tags), string(keywords), profile.GroupId,
		createdAt, updatedAt, NormalizeRestoreLastSessionMode(profile.RestoreLastSession), profile.DeletedAt,
	)
	if err != nil {
		return "", "", fmt.Errorf("保存实例配置失败: %w", err)
	}
	return createdAt, updatedAt, nil
}

// SoftDelete 将实例移入回收站
func (d *SQLiteProfileDAO) SoftDelete(profileId string, deletedAt string) error {
	result, err := d.db.Exec(`UPDATE browser_profiles SET deleted_at = ?, updated_at = ? WHERE profile_id = ?`, deletedAt, deletedAt, profileId)
	if err != nil {
		return fmt.Errorf("移入回收站失败: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("实例不存在: %s", profileId)
	}
	return nil
}

// Restore 从回收站恢复实例
func (d *SQLiteProfileDAO) Restore(profileId string) error {
	now := time.Now().Format(time.RFC3339)
	result, err := d.db.Exec(`UPDATE browser_profiles SET deleted_at = '', updated_at = ? WHERE profile_id = ?`, now, profileId)
	if err != nil {
		return fmt.Errorf("恢复实例失败: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("实例不存在: %s", profileId)
	}
	return nil
}

// ListExpiredDeleted 查询超过保留期的回收站实例
func (d *SQLiteProfileDAO) ListExpiredDeleted(expiredBefore string) ([]*Profile, error) {
	rows, err := d.db.Query(`
		SELECT profile_id, profile_name, user_data_dir, core_id,
		       fingerprint_args, proxy_id, proxy_config,
		       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
		       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
		       COALESCE(memory_limit_mb, 0),
		       launch_args,
		       tags, keywords, group_id, created_at, updated_at,
		       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
		FROM browser_profiles WHERE COALESCE(deleted_at, '') != '' AND deleted_at <= ?`, expiredBefore)
	if err != nil {
		return nil, fmt.Errorf("查询过期回收站实例失败: %w", err)
	}
	var expired []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		expired = append(expired, p)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return expired, nil
}

// Delete 删除实例配置
func (d *SQLiteProfileDAO) Delete(profileId string) error {
	_, err := d.db.Exec(`DELETE FROM browser_profiles WHERE profile_id = ?`, profileId)
	if err != nil {
		return fmt.Errorf("删除实例配置失败: %w", err)
	}
	return nil
}

// ListByGroup 按分组筛选实例
// groupId 为空字符串时返回未分组的实例
// includeChildren=true 时同时包含 childGroupIds 中的子分组实例
func (d *SQLiteProfileDAO) ListByGroup(groupId string, includeChildren bool, childGroupIds []string) ([]*Profile, error) {
	var rows *sql.Rows
	var err error

	if includeChildren && len(childGroupIds) > 0 {
		// 构建 IN 子句，包含当前分组和所有子分组
		allIds := append([]string{groupId}, childGroupIds...)
		inClause := ""
		args := make([]interface{}, len(allIds))
		for i, id := range allIds {
			if i > 0 {
				inClause += ","
			}
			inClause += "?"
			args[i] = id
		}
		rows, err = d.db.Query(fmt.Sprintf(`
			SELECT profile_id, profile_name, user_data_dir, core_id,
			       fingerprint_args, proxy_id, proxy_config,
			       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
			       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
			       COALESCE(memory_limit_mb, 0),
			       launch_args,
			       tags, keywords, group_id, created_at, updated_at,
			       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
			FROM browser_profiles WHERE COALESCE(deleted_at, '') = '' AND group_id IN (%s) ORDER BY created_at ASC`, inClause), args...)
	} else {
		// 仅查询指定分组
		rows, err = d.db.Query(`
			SELECT profile_id, profile_name, user_data_dir, core_id,
			       fingerprint_args, proxy_id, proxy_config,
			       COALESCE(proxy_bind_source_id, ''), COALESCE(proxy_bind_source_url, ''),
			       COALESCE(proxy_bind_name, ''), COALESCE(proxy_bind_updated_at, ''),
			       COALESCE(memory_limit_mb, 0),
			       launch_args,
			       tags, keywords, group_id, created_at, updated_at,
			       COALESCE(restore_last_session, ''), COALESCE(deleted_at, '')
			FROM browser_profiles WHERE COALESCE(deleted_at, '') = '' AND group_id = ? ORDER BY created_at ASC`, groupId)
	}

	if err != nil {
		return nil, fmt.Errorf("按分组查询实例失败: %w", err)
	}
	defer rows.Close()

	var list []*Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// MoveToGroup 批量移动实例到分组，返回与数据库一致的更新时间供内存态同步。
func (d *SQLiteProfileDAO) MoveToGroup(profileIds []string, groupId string) (string, error) {
	if len(profileIds) == 0 {
		return "", nil
	}
	updatedAt := time.Now().Format(time.RFC3339)
	inClause := ""
	args := make([]interface{}, len(profileIds)+2)
	args[0] = groupId
	args[1] = updatedAt
	for i, id := range profileIds {
		if i > 0 {
			inClause += ","
		}
		inClause += "?"
		args[i+2] = id
	}
	_, err := d.db.Exec(fmt.Sprintf(`UPDATE browser_profiles SET group_id = ?, updated_at = ? WHERE profile_id IN (%s)`, inClause), args...)
	if err != nil {
		return "", fmt.Errorf("批量移动实例失败: %w", err)
	}
	return updatedAt, nil
}

// scanner 统一扫描接口，兼容 *sql.Row 和 *sql.Rows
type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(s scanner) (*Profile, error) {
	var (
		fingerprintArgsJSON, launchArgsJSON, tagsJSON, keywordsJSON string
		p                                                           Profile
	)
	err := s.Scan(
		&p.ProfileId, &p.ProfileName, &p.UserDataDir, &p.CoreId,
		&fingerprintArgsJSON, &p.ProxyId, &p.ProxyConfig,
		&p.ProxyBindSourceID, &p.ProxyBindSourceURL, &p.ProxyBindName, &p.ProxyBindUpdatedAt,
		&p.MemoryLimitMB, &launchArgsJSON, &tagsJSON, &keywordsJSON, &p.GroupId,
		&p.CreatedAt, &p.UpdatedAt, &p.RestoreLastSession, &p.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(fingerprintArgsJSON), &p.FingerprintArgs)
	_ = json.Unmarshal([]byte(launchArgsJSON), &p.LaunchArgs)
	_ = json.Unmarshal([]byte(tagsJSON), &p.Tags)
	_ = json.Unmarshal([]byte(keywordsJSON), &p.Keywords)
	if p.FingerprintArgs == nil {
		p.FingerprintArgs = []string{}
	}
	if p.LaunchArgs == nil {
		p.LaunchArgs = []string{}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Keywords == nil {
		p.Keywords = []string{}
	}
	p.RestoreLastSession = NormalizeRestoreLastSessionMode(p.RestoreLastSession)
	p.MemoryLimitMB = normalizeMemoryLimitMB(p.MemoryLimitMB)
	return &p, nil
}
