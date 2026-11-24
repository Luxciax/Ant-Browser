package backend

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultBatchConcurrency = 3
const maxBatchConcurrency = 8
const maxBatchItems = 100

type BrowserBatchItemResult struct {
	Index       int             `json:"index"`
	ClientKey   string          `json:"clientKey,omitempty"`
	ProfileID   string          `json:"profileId,omitempty"`
	ProfileName string          `json:"profileName,omitempty"`
	OK          bool            `json:"ok"`
	Error       string          `json:"error,omitempty"`
	Runtime     *BrowserProfile `json:"runtime,omitempty"`
}

type BrowserBatchOperation struct {
	OperationID string                   `json:"operationId"`
	Kind        string                   `json:"kind"`
	Status      string                   `json:"status"`
	Concurrency int                      `json:"concurrency"`
	Total       int                      `json:"total"`
	Succeeded   int                      `json:"succeeded"`
	Failed      int                      `json:"failed"`
	StartedAt   string                   `json:"startedAt"`
	FinishedAt  string                   `json:"finishedAt,omitempty"`
	Results     []BrowserBatchItemResult `json:"results"`
}

type BrowserBatchCreateItem struct {
	ClientKey string              `json:"clientKey,omitempty"`
	Config    BrowserProfileInput `json:"config"`
}

type BrowserBatchCreateRequest struct {
	Items       []BrowserBatchCreateItem `json:"items"`
	Concurrency int                      `json:"concurrency,omitempty"`
}

type BrowserBatchProfilesRequest struct {
	ProfileIDs  []string `json:"profileIds"`
	Concurrency int      `json:"concurrency,omitempty"`
}

func normalizeBatchConcurrency(value int) int {
	if value <= 0 { return defaultBatchConcurrency }
	if value > maxBatchConcurrency { return maxBatchConcurrency }
	return value
}

func validateBatchSize(total int) error {
	if total <= 0 { return fmt.Errorf("batch items are required") }
	if total > maxBatchItems { return fmt.Errorf("batch item count exceeds %d", maxBatchItems) }
	return nil
}

func cloneBatchOperation(op BrowserBatchOperation) BrowserBatchOperation {
	out := op
	out.Results = append([]BrowserBatchItemResult{}, op.Results...)
	for i := range out.Results {
		if op.Results[i].Runtime != nil {
			out.Results[i].Runtime = copyBrowserProfileSnapshot(op.Results[i].Runtime)
		}
	}
	return out
}

func (a *App) storeBatchOperation(op BrowserBatchOperation) {
	a.batchMu.Lock()
	if a.batchOperations == nil { a.batchOperations = make(map[string]BrowserBatchOperation) }
	a.batchOperations[op.OperationID] = cloneBatchOperation(op)
	_ = a.persistBatchOperationsLocked()
	a.batchMu.Unlock()
}

func (a *App) BrowserBatchOperationGet(operationID string) (*BrowserBatchOperation, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" { return nil, fmt.Errorf("operation id is required") }
	a.batchMu.RLock()
	op, ok := a.batchOperations[operationID]
	a.batchMu.RUnlock()
	if !ok { return nil, fmt.Errorf("batch operation not found") }
	copyOp := cloneBatchOperation(op)
	return &copyOp, nil
}

func runBatch(total, concurrency int, fn func(index int) BrowserBatchItemResult) []BrowserBatchItemResult {
	results := make([]BrowserBatchItemResult, total)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = fn(index)
			}
		}()
	}
	for index := 0; index < total; index++ { jobs <- index }
	close(jobs)
	wg.Wait()
	return results
}

func finalizeBatch(op *BrowserBatchOperation) {
	op.Status = "completed"
	op.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	op.Succeeded, op.Failed = 0, 0
	for _, item := range op.Results {
		if item.OK { op.Succeeded++ } else { op.Failed++ }
	}
	if op.Failed > 0 && op.Succeeded > 0 { op.Status = "partial" }
	if op.Failed == op.Total { op.Status = "failed" }
}

func (a *App) BrowserBatchCreate(req BrowserBatchCreateRequest) (*BrowserBatchOperation, error) {
	if err := validateBatchSize(len(req.Items)); err != nil { return nil, err }
	concurrency := normalizeBatchConcurrency(req.Concurrency)
	op := BrowserBatchOperation{
		OperationID: uuid.NewString(), Kind: "create", Status: "running",
		Concurrency: concurrency, Total: len(req.Items),
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	a.storeBatchOperation(op)
	op.Results = runBatch(len(req.Items), concurrency, func(index int) BrowserBatchItemResult {
		item := req.Items[index]
		result := BrowserBatchItemResult{Index:index, ClientKey:strings.TrimSpace(item.ClientKey)}
		profile, err := a.BrowserProfileCreate(item.Config)
		if err != nil { result.Error = err.Error(); return result }
		result.OK = true
		result.ProfileID = profile.ProfileId
		result.ProfileName = profile.ProfileName
		result.Runtime = profile
		return result
	})
	finalizeBatch(&op)
	a.storeBatchOperation(op)
	copyOp := cloneBatchOperation(op)
	return &copyOp, nil
}

func normalizeBatchProfileIDs(values []string) ([]string, error) {
	if err := validateBatchSize(len(values)); err != nil { return nil, err }
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" { continue }
		if _, ok := seen[value]; ok { continue }
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 { return nil, fmt.Errorf("profile ids are required") }
	return out, nil
}

func (a *App) runBatchProfiles(kind string, req BrowserBatchProfilesRequest, action func(string) (*BrowserProfile, error)) (*BrowserBatchOperation, error) {
	ids, err := normalizeBatchProfileIDs(req.ProfileIDs)
	if err != nil { return nil, err }
	concurrency := normalizeBatchConcurrency(req.Concurrency)
	op := BrowserBatchOperation{
		OperationID: uuid.NewString(), Kind: kind, Status: "running",
		Concurrency: concurrency, Total: len(ids),
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	a.storeBatchOperation(op)
	op.Results = runBatch(len(ids), concurrency, func(index int) BrowserBatchItemResult {
		profileID := ids[index]
		result := BrowserBatchItemResult{Index:index, ProfileID:profileID}
		profile, actionErr := action(profileID)
		if actionErr != nil {
			result.Error = actionErr.Error()
			if current, statusErr := a.BrowserInstanceStatus(profileID); statusErr == nil {
				result.Runtime = current
				result.ProfileName = current.ProfileName
			}
			return result
		}
		result.OK = true
		result.Runtime = profile
		if profile != nil { result.ProfileName = profile.ProfileName }
		return result
	})
	finalizeBatch(&op)
	a.storeBatchOperation(op)
	copyOp := cloneBatchOperation(op)
	return &copyOp, nil
}

func (a *App) BrowserBatchStart(req BrowserBatchProfilesRequest) (*BrowserBatchOperation, error) {
	return a.runBatchProfiles("start", req, a.BrowserInstanceStart)
}

func (a *App) BrowserBatchStop(req BrowserBatchProfilesRequest) (*BrowserBatchOperation, error) {
	return a.runBatchProfiles("stop", req, a.BrowserInstanceStop)
}
