package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const WorkflowSchemaVersion = 1

type WorkflowStore struct {
	definitionsDir string
	runsDir        string
}

func NewWorkflowStore(root string) *WorkflowStore {
	root = filepath.Clean(strings.TrimSpace(root))
	return &WorkflowStore{
		definitionsDir: filepath.Join(root, "workflows"),
		runsDir:        filepath.Join(root, "workflow-runs"),
	}
}

func (s *WorkflowStore) ensure() error {
	if s == nil { return fmt.Errorf("workflow store is unavailable") }
	if err := os.MkdirAll(s.definitionsDir, 0o755); err != nil { return err }
	if err := os.MkdirAll(s.runsDir, 0o755); err != nil { return err }
	return nil
}

func safeWorkflowID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" { return "", fmt.Errorf("workflow id is required") }
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", fmt.Errorf("workflow id contains unsupported characters")
		}
	}
	return value, nil
}

func (s *WorkflowStore) SaveDefinition(input WorkflowDefinition) (WorkflowDefinition, error) {
	if err := s.ensure(); err != nil { return WorkflowDefinition{}, err }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(input.ID) == "" { input.ID = uuid.NewString() }
	id, err := safeWorkflowID(input.ID); if err != nil { return WorkflowDefinition{}, err }
	input.ID = id
	input.SchemaVersion = WorkflowSchemaVersion
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" { return WorkflowDefinition{}, fmt.Errorf("workflow name is required") }
	if input.CreatedAt == "" { input.CreatedAt = now }
	input.UpdatedAt = now
	if input.Variables == nil { input.Variables = map[string]any{} }
	if input.Steps == nil { input.Steps = []WorkflowStep{} }
	data, err := json.MarshalIndent(input, "", "  "); if err != nil { return WorkflowDefinition{}, err }
	if err := writeFileAtomic(filepath.Join(s.definitionsDir, id+".json"), data, 0o600); err != nil { return WorkflowDefinition{}, err }
	return input, nil
}

func (s *WorkflowStore) GetDefinition(id string) (WorkflowDefinition, error) {
	if err := s.ensure(); err != nil { return WorkflowDefinition{}, err }
	id, err := safeWorkflowID(id); if err != nil { return WorkflowDefinition{}, err }
	data, err := os.ReadFile(filepath.Join(s.definitionsDir, id+".json")); if err != nil { return WorkflowDefinition{}, err }
	var out WorkflowDefinition
	if err := json.Unmarshal(data, &out); err != nil { return WorkflowDefinition{}, err }
	return out, nil
}

func (s *WorkflowStore) ListDefinitions() ([]WorkflowDefinition, error) {
	if err := s.ensure(); err != nil { return nil, err }
	entries, err := os.ReadDir(s.definitionsDir); if err != nil { return nil, err }
	out := []WorkflowDefinition{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") { continue }
		data, err := os.ReadFile(filepath.Join(s.definitionsDir, e.Name())); if err != nil { continue }
		var item WorkflowDefinition
		if json.Unmarshal(data, &item) == nil { out = append(out, item) }
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out, nil
}

func (s *WorkflowStore) DeleteDefinition(id string) error {
	id, err := safeWorkflowID(id); if err != nil { return err }
	if err := os.Remove(filepath.Join(s.definitionsDir, id+".json")); err != nil && !os.IsNotExist(err) { return err }
	return nil
}

func (s *WorkflowStore) SaveRun(input WorkflowRunRecord) (WorkflowRunRecord, error) {
	if err := s.ensure(); err != nil { return WorkflowRunRecord{}, err }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(input.ID) == "" { input.ID = uuid.NewString() }
	input.SchemaVersion = WorkflowSchemaVersion
	if input.StartedAt == "" { input.StartedAt = now }
	input.UpdatedAt = now
	if input.Variables == nil { input.Variables = map[string]any{} }
	if input.Steps == nil { input.Steps = []WorkflowRunStepRecord{} }
	data, err := json.MarshalIndent(input, "", "  "); if err != nil { return WorkflowRunRecord{}, err }
	if err := writeFileAtomic(filepath.Join(s.runsDir, input.ID+".json"), data, 0o600); err != nil { return WorkflowRunRecord{}, err }
	return input, nil
}

func (s *WorkflowStore) GetRun(id string) (WorkflowRunRecord, error) {
	if err := s.ensure(); err != nil { return WorkflowRunRecord{}, err }
	id, err := safeWorkflowID(id); if err != nil { return WorkflowRunRecord{}, err }
	data, err := os.ReadFile(filepath.Join(s.runsDir, id+".json")); if err != nil { return WorkflowRunRecord{}, err }
	var out WorkflowRunRecord
	if err := json.Unmarshal(data, &out); err != nil { return WorkflowRunRecord{}, err }
	return out, nil
}

func (s *WorkflowStore) ListRuns(limit int) ([]WorkflowRunRecord, error) {
	if err := s.ensure(); err != nil { return nil, err }
	if limit <= 0 { limit = 50 }
	if limit > 500 { limit = 500 }
	entries, err := os.ReadDir(s.runsDir); if err != nil { return nil, err }
	out := []WorkflowRunRecord{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") { continue }
		data, err := os.ReadFile(filepath.Join(s.runsDir, e.Name())); if err != nil { continue }
		var item WorkflowRunRecord
		if json.Unmarshal(data, &item) == nil { out = append(out, item) }
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if len(out) > limit { out = out[:limit] }
	return out, nil
}

func (s *WorkflowStore) RecoverInterruptedRuns() error {
	items, err := s.ListRuns(500); if err != nil { return err }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range items {
		if item.Status != "running" { continue }
		item.Status = "interrupted"
		item.FinishedAt = now
		item.UpdatedAt = now
		if _, err := s.SaveRun(item); err != nil { return err }
	}
	return nil
}
