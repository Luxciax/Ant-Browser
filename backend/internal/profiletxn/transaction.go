// Package profiletxn coordinates profile directory changes with SQLite commits.
// A durable intent precedes all renames. The commit bit is written in the same
// SQL transaction as the business rows, never inferred from directory names.
package profiletxn

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Move struct {
	Source      string `json:"source,omitempty"` // Empty for deletion.
	Target      string `json:"target"`
	HadOriginal bool   `json:"hadOriginal"`
}

type Plan struct {
	Kind        string `json:"kind"`
	Moves       []Move `json:"moves"`
	StagingRoot string `json:"stagingRoot,omitempty"`
}

type Transaction struct {
	ID   string
	Plan Plan
	db   *sql.DB
}

var ErrCommitUncertain = errors.New("实例数据库提交结果无法确认，请重启恢复")

func Begin(db *sql.DB, plan Plan, roots []string) (*Transaction, error) {
	t := &Transaction{ID: uuid.NewString(), Plan: plan, db: db}
	if err := t.validate(roots); err != nil {
		return nil, err
	}
	for i := range t.Plan.Moves {
		m := &t.Plan.Moves[i]
		var err error
		m.HadOriginal, err = exists(m.Target)
		if err != nil {
			return nil, err
		}
		if occupied, err := exists(t.backup(*m)); err != nil || occupied {
			return nil, fmt.Errorf("transaction backup already exists or is inaccessible: %s: %v", t.backup(*m), err)
		}
		if m.Source != "" {
			if present, err := exists(m.Source); err != nil || !present {
				return nil, fmt.Errorf("staging directory missing: %s: %v", m.Source, err)
			}
		}
	}
	data, err := json.Marshal(t.Plan)
	if err != nil {
		return nil, err
	}
	result, err := db.Exec(`INSERT INTO profile_file_transactions(id, plan, committed) SELECT ?, ?, 0 WHERE NOT EXISTS (SELECT 1 FROM profile_file_transactions)`, t.ID, string(data))
	if err != nil {
		return nil, err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return nil, fmt.Errorf("存在未完成的实例文件事务，请重启完成恢复: %v", err)
	}
	return t, nil
}

func (t *Transaction) backup(m Move) string { return m.Target + ".ant-tx-" + t.ID }

func (t *Transaction) Apply() error {
	for _, m := range t.Plan.Moves {
		if err := os.MkdirAll(filepath.Dir(m.Target), 0755); err != nil {
			return err
		}
		if m.HadOriginal {
			if err := os.Rename(m.Target, t.backup(m)); err != nil {
				return err
			}
		}
		if m.Source != "" {
			if err := os.Rename(m.Source, m.Target); err != nil {
				return err
			}
		}
	}
	return nil
}

// MarkCommitted must use the SQL transaction that changes the profile rows.
func (t *Transaction) MarkCommitted(tx *sql.Tx) error {
	result, err := tx.Exec(`UPDATE profile_file_transactions SET committed = 1 WHERE id = ?`, t.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return fmt.Errorf("实例事务记录缺失: %v", err)
	}
	return nil
}

// Commit resolves an ambiguous driver result by reading the durable bit. If
// SQLite cannot be read, the caller retains the intent and files for recovery.
func (t *Transaction) Commit(tx *sql.Tx) error {
	if err := t.MarkCommitted(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		var committed bool
		readErr := t.db.QueryRow(`SELECT committed FROM profile_file_transactions WHERE id = ?`, t.ID).Scan(&committed)
		if readErr == nil && committed {
			return nil
		}
		if readErr != nil {
			return errors.Join(ErrCommitUncertain, err, readErr)
		}
		return errors.Join(err, readErr)
	}
	return nil
}

// Resolve is idempotent, including interruption during recovery itself. A DB
// read failure preserves all files: an uncertain commit must never be guessed.
func (t *Transaction) Resolve() (bool, error) {
	var committed bool
	if err := t.db.QueryRow(`SELECT committed FROM profile_file_transactions WHERE id = ?`, t.ID).Scan(&committed); err != nil {
		return false, err
	}
	for i := len(t.Plan.Moves) - 1; i >= 0; i-- {
		m := t.Plan.Moves[i]
		backup := t.backup(m)
		hasBackup, err := exists(backup)
		if err != nil {
			return committed, err
		}
		hasTarget, err := exists(m.Target)
		if err != nil {
			return committed, err
		}
		if committed {
			if m.Source != "" && !hasTarget {
				return true, fmt.Errorf("已提交的实例目录缺失，保留恢复备份: %s", m.Target)
			}
			if err := os.RemoveAll(backup); err != nil {
				return true, err
			}
			continue
		}
		if hasBackup {
			if m.Source == "" && hasTarget {
				return false, fmt.Errorf("恢复目标已存在，保留两份目录: %s", m.Target)
			}
			if m.Source != "" {
				if err := os.RemoveAll(m.Target); err != nil {
					return false, err
				}
			}
			if err := os.Rename(backup, m.Target); err != nil {
				return false, err
			}
		} else if m.HadOriginal {
			if !hasTarget {
				return false, fmt.Errorf("原目录和事务备份均缺失: %s", m.Target)
			}
		} else if m.Source != "" {
			// Source still present means this move was never applied. Otherwise
			// Target is the uncommitted newly imported directory.
			hasSource, err := exists(m.Source)
			if err != nil {
				return false, err
			}
			if !hasSource {
				if err := os.RemoveAll(m.Target); err != nil {
					return false, err
				}
			}
		}
	}
	if t.Plan.StagingRoot != "" {
		if err := os.RemoveAll(t.Plan.StagingRoot); err != nil {
			return committed, err
		}
	}
	_, err := t.db.Exec(`DELETE FROM profile_file_transactions WHERE id = ?`, t.ID)
	return committed, err
}

// Recover runs before managers, API servers and automatic trash cleanup start.
func Recover(db *sql.DB, roots []string) error {
	rows, err := db.Query(`SELECT id, plan FROM profile_file_transactions ORDER BY id`)
	if err != nil {
		return err
	}
	var pending []*Transaction
	for rows.Next() {
		t := &Transaction{db: db}
		var data string
		if err := rows.Scan(&t.ID, &data); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal([]byte(data), &t.Plan); err != nil {
			rows.Close()
			return err
		}
		if err := t.validate(roots); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, t)
	}
	err = errors.Join(rows.Err(), rows.Close()) // Close before using the single DB connection.
	if err != nil {
		return err
	}
	for _, t := range pending {
		if _, err := t.Resolve(); err != nil {
			return fmt.Errorf("实例事务 %s 恢复失败: %w", t.ID, err)
		}
	}
	return nil
}

func (t *Transaction) validate(roots []string) error {
	if _, err := uuid.Parse(t.ID); err != nil {
		return err
	}
	if t.Plan.Kind != "import" && t.Plan.Kind != "delete" {
		return fmt.Errorf("未知实例事务类型: %s", t.Plan.Kind)
	}
	allowed := func(path string) bool {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return false
		}
		resolved, err := physicalPath(path)
		if err != nil {
			return false
		}
		for _, root := range roots {
			resolvedRoot, err := physicalPath(root)
			if err == nil && inside(path, root) && inside(resolved, resolvedRoot) {
				return true
			}
		}
		return false
	}
	if t.Plan.StagingRoot != "" && !allowed(t.Plan.StagingRoot) {
		return fmt.Errorf("事务暂存路径越界")
	}
	for i, m := range t.Plan.Moves {
		if !allowed(m.Target) {
			return fmt.Errorf("事务目标路径越界: %s", m.Target)
		}
		if t.Plan.Kind == "import" && (m.Source == "" || !allowed(m.Source) || !inside(m.Source, t.Plan.StagingRoot)) {
			return fmt.Errorf("事务源路径越界: %s", m.Source)
		}
		if t.Plan.Kind == "delete" && (m.Source != "" || t.Plan.StagingRoot != "") {
			return fmt.Errorf("删除事务包含导入路径")
		}
		if t.Plan.StagingRoot != "" && (inside(m.Target, t.Plan.StagingRoot) || inside(t.Plan.StagingRoot, m.Target) || strings.EqualFold(m.Target, t.Plan.StagingRoot)) {
			return fmt.Errorf("事务暂存与目标重叠")
		}
		for _, previous := range t.Plan.Moves[:i] {
			if strings.EqualFold(previous.Target, m.Target) || inside(previous.Target, m.Target) || inside(m.Target, previous.Target) {
				return fmt.Errorf("事务目录重叠")
			}
		}
	}
	return nil
}

// Resolve existing ancestors too: the final staging/backup path may not yet
// exist, but a junction in its parent must not escape the configured roots.
func physicalPath(path string) (string, error) {
	candidate := path
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", err
		}
		tail = append(tail, filepath.Base(candidate))
		candidate = parent
	}
}

func inside(path, root string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
