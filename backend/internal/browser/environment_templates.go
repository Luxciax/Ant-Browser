package browser

import (
	"fmt"
	"strings"
	"time"
)

const EnvironmentTemplateSchemaVersion = 1

type EnvironmentTemplateRule struct {
	Key         string `json:"key"`
	Enforcement string `json:"enforcement"`
	Description string `json:"description"`
}

type EnvironmentTemplate struct {
	SchemaVersion       int                       `json:"schemaVersion"`
	ID                  string                    `json:"id"`
	Revision            string                    `json:"revision"`
	Name                string                    `json:"name"`
	Description         string                    `json:"description"`
	RestoreLastSession  string                    `json:"restoreLastSession"`
	MemoryLimitMB       int                       `json:"memoryLimitMb"`
	Rules               []EnvironmentTemplateRule `json:"rules"`
}

func builtinEnvironmentTemplates() []EnvironmentTemplate {
	commonRules := []EnvironmentTemplateRule{
		{Key: "kernel-resolvable", Enforcement: "hard", Description: "selected/default kernel must resolve to an executable"},
		{Key: "managed-launch-args", Enforcement: "hard", Description: "application-owned launch arguments cannot be overridden by external input"},
		{Key: "proxy-fail-closed", Enforcement: "hard", Description: "configured proxy failures do not silently fall back to direct"},
		{Key: "runtime-evidence", Enforcement: "soft", Description: "explicit runtime verification is recommended after material configuration changes"},
		{Key: "locale-link-explicit", Enforcement: "soft", Description: "network-derived locale recommendations require an explicit policy"},
	}
	return []EnvironmentTemplate{
		{
			SchemaVersion: EnvironmentTemplateSchemaVersion,
			ID: "balanced-workspace", Revision: "1",
			Name: "Balanced Workspace",
			Description: "General-purpose managed environment with inherited session policy and no artificial memory cap.",
			RestoreLastSession: RestoreLastSessionFollow,
			MemoryLimitMB: 0,
			Rules: append([]EnvironmentTemplateRule{}, commonRules...),
		},
		{
			SchemaVersion: EnvironmentTemplateSchemaVersion,
			ID: "persistent-workspace", Revision: "1",
			Name: "Persistent Workspace",
			Description: "Keeps Chromium session restoration enabled while retaining the same safety rules.",
			RestoreLastSession: RestoreLastSessionEnabled,
			MemoryLimitMB: 0,
			Rules: append([]EnvironmentTemplateRule{}, commonRules...),
		},
		{
			SchemaVersion: EnvironmentTemplateSchemaVersion,
			ID: "constrained-runner", Revision: "1",
			Name: "Constrained Runner",
			Description: "Fresh-session operational template with a conservative 2048 MB process memory ceiling.",
			RestoreLastSession: RestoreLastSessionDisabled,
			MemoryLimitMB: 2048,
			Rules: append([]EnvironmentTemplateRule{}, commonRules...),
		},
	}
}

func BuiltinEnvironmentTemplates() []EnvironmentTemplate {
	items := builtinEnvironmentTemplates()
	out := make([]EnvironmentTemplate, len(items))
	for i := range items {
		out[i] = items[i]
		out[i].Rules = append([]EnvironmentTemplateRule{}, items[i].Rules...)
	}
	return out
}

func FindEnvironmentTemplate(id string) (EnvironmentTemplate, bool) {
	id = strings.TrimSpace(id)
	for _, item := range builtinEnvironmentTemplates() {
		if strings.EqualFold(item.ID, id) {
			item.Rules = append([]EnvironmentTemplateRule{}, item.Rules...)
			return item, true
		}
	}
	return EnvironmentTemplate{}, false
}

func (m *Manager) ApplyEnvironmentTemplate(profileID, templateID string) (*Profile, error) {
	if m == nil {
		return nil, fmt.Errorf("browser manager is unavailable")
	}
	template, ok := FindEnvironmentTemplate(templateID)
	if !ok {
		return nil, fmt.Errorf("environment template not found")
	}
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	profile := m.Profiles[strings.TrimSpace(profileID)]
	if profile == nil {
		return nil, fmt.Errorf("profile not found")
	}
	if ProfileRuntimeMutationBlocked(profile) {
		return nil, fmt.Errorf("profile runtime is %s; stop it before applying a template", NormalizeProfileRuntimeState(profile))
	}
	previous := cloneProfileValue(profile)
	next := cloneProfileValue(profile)
	next.RestoreLastSession = NormalizeRestoreLastSessionMode(template.RestoreLastSession)
	next.MemoryLimitMB = normalizeMemoryLimitMB(template.MemoryLimitMB)
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.Upsert(&next); err != nil {
			return nil, err
		}
		*profile = next
	} else {
		*profile = next
		if err := m.SaveProfiles(); err != nil {
			*profile = previous
			return nil, err
		}
	}
	if m.TemplateBindingDAO != nil {
		binding := EnvironmentTemplateBinding{
			ProfileID: profile.ProfileId,
			TemplateID: template.ID,
			TemplateRevision: template.Revision,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err := m.TemplateBindingDAO.Upsert(binding); err != nil {
			*profile = previous
			if m.ProfileDAO != nil {
				_ = m.ProfileDAO.Upsert(profile)
			} else {
				_ = m.SaveProfiles()
			}
			return nil, err
		}
	}
	snapshot := cloneProfileValue(profile)
	return &snapshot, nil
}
