package browser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type EnvironmentTemplateBinding struct {
	ProfileID        string `json:"profileId"`
	TemplateID       string `json:"templateId"`
	TemplateRevision string `json:"templateRevision"`
	UpdatedAt        string `json:"updatedAt"`
}

type EnvironmentTemplateBindingDAO interface {
	Get(profileID string) (EnvironmentTemplateBinding, error)
	Upsert(binding EnvironmentTemplateBinding) error
	Delete(profileID string) error
}

type SQLiteEnvironmentTemplateBindingDAO struct {
	db *sql.DB
}

func NewSQLiteEnvironmentTemplateBindingDAO(db *sql.DB) *SQLiteEnvironmentTemplateBindingDAO {
	return &SQLiteEnvironmentTemplateBindingDAO{db: db}
}

func (d *SQLiteEnvironmentTemplateBindingDAO) Get(profileID string) (EnvironmentTemplateBinding, error) {
	if d == nil || d.db == nil {
		return EnvironmentTemplateBinding{}, fmt.Errorf("environment template database is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return EnvironmentTemplateBinding{}, fmt.Errorf("profile id is required")
	}
	var out EnvironmentTemplateBinding
	err := d.db.QueryRow(`
		SELECT profile_id, template_id, template_revision, updated_at
		FROM browser_profile_environment_template
		WHERE profile_id = ?
	`, profileID).Scan(&out.ProfileID, &out.TemplateID, &out.TemplateRevision, &out.UpdatedAt)
	return out, err
}

func (d *SQLiteEnvironmentTemplateBindingDAO) Upsert(binding EnvironmentTemplateBinding) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("environment template database is unavailable")
	}
	binding.ProfileID = strings.TrimSpace(binding.ProfileID)
	binding.TemplateID = strings.TrimSpace(binding.TemplateID)
	binding.TemplateRevision = strings.TrimSpace(binding.TemplateRevision)
	if binding.ProfileID == "" || binding.TemplateID == "" || binding.TemplateRevision == "" {
		return fmt.Errorf("profile id, template id and template revision are required")
	}
	if binding.UpdatedAt == "" {
		binding.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := d.db.Exec(`
		INSERT INTO browser_profile_environment_template
			(profile_id, template_id, template_revision, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(profile_id) DO UPDATE SET
			template_id = excluded.template_id,
			template_revision = excluded.template_revision,
			updated_at = excluded.updated_at
	`, binding.ProfileID, binding.TemplateID, binding.TemplateRevision, binding.UpdatedAt)
	return err
}

func (d *SQLiteEnvironmentTemplateBindingDAO) Delete(profileID string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("environment template database is unavailable")
	}
	_, err := d.db.Exec("DELETE FROM browser_profile_environment_template WHERE profile_id = ?", strings.TrimSpace(profileID))
	return err
}
