package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ant-chrome/backend/internal/fsutil"
)

const (
	batchOperationStoreVersion = 1
	batchOperationHistoryLimit = 200
)

type batchOperationStoreFile struct {
	Version    int                     `json:"version"`
	Operations []BrowserBatchOperation `json:"operations"`
}

func (a *App) batchOperationStorePath() string {
	if a == nil {
		return ""
	}
	return a.resolveAppPath(filepath.Join("data", "batch_operations.v1.json"))
}

func sanitizeBatchOperationForStore(op BrowserBatchOperation) BrowserBatchOperation {
	out := cloneBatchOperation(op)
	for index := range out.Results {
		// Persist only the public/runtime-safe profile snapshot already returned
		// by batch APIs. Never add proxy credentials or launch internals here.
		if out.Results[index].Runtime != nil {
			runtime := out.Results[index].Runtime
			runtime.ProxyConfig = ""
			runtime.ProxyBindSourceURL = ""
			runtime.ProxyBindSourceID = ""
			runtime.LaunchCode = ""
			runtime.WindowMarkerCode = ""
			runtime.UserDataDir = ""
			runtime.FingerprintArgs = nil
			runtime.LaunchArgs = nil
			runtime.LastLaunchArgs = nil
		}
	}
	return out
}

func (a *App) persistBatchOperationsLocked() error {
	if a == nil {
		return fmt.Errorf("application is unavailable")
	}
	items := make([]BrowserBatchOperation, 0, len(a.batchOperations))
	for _, op := range a.batchOperations {
		items = append(items, sanitizeBatchOperationForStore(op))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt > items[j].StartedAt
	})
	if len(items) > batchOperationHistoryLimit {
		items = items[:batchOperationHistoryLimit]
	}
	payload, err := json.MarshalIndent(batchOperationStoreFile{Version: batchOperationStoreVersion, Operations: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode batch operation store: %w", err)
	}
	path := a.batchOperationStorePath()
	if path == "" {
		return fmt.Errorf("batch operation store path is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create batch operation directory: %w", err)
	}
	if err := fsutil.AtomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write batch operation store: %w", err)
	}
	return nil
}

func (a *App) loadBatchOperations() error {
	if a == nil {
		return nil
	}
	path := a.batchOperationStorePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read batch operation store: %w", err)
	}
	var stored batchOperationStoreFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("decode batch operation store: %w", err)
	}
	if stored.Version != batchOperationStoreVersion {
		return fmt.Errorf("unsupported batch operation store version")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	a.batchMu.Lock()
	defer a.batchMu.Unlock()
	if a.batchOperations == nil {
		a.batchOperations = make(map[string]BrowserBatchOperation)
	}
	changed := false
	for _, op := range stored.Operations {
		if strings.TrimSpace(op.OperationID) == "" {
			continue
		}
		if op.Status == "running" {
			op.Status = "interrupted"
			op.FinishedAt = now
			changed = true
		}
		a.batchOperations[op.OperationID] = cloneBatchOperation(op)
	}
	if changed {
		return a.persistBatchOperationsLocked()
	}
	return nil
}
