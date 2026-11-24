package backend

import "testing"

func TestBatchOperationPersistenceRecoversRunningAsInterrupted(t *testing.T) {
	app := NewApp(t.TempDir())
	app.batchOperations["op-1"] = BrowserBatchOperation{
		OperationID: "op-1",
		Kind: "start",
		Status: "running",
		Total: 2,
		Results: []BrowserBatchItemResult{
			{Index: 0, Completed: true, OK: true, ProfileID: "p1"},
			{Index: 1, Completed: false, ProfileID: "p2"},
		},
	}
	app.batchMu.Lock()
	if err := app.persistBatchOperationsLocked(); err != nil {
		app.batchMu.Unlock()
		t.Fatalf("persistBatchOperationsLocked: %v", err)
	}
	app.batchMu.Unlock()

	reloaded := NewApp(app.appRoot)
	if err := reloaded.loadBatchOperations(); err != nil {
		t.Fatalf("loadBatchOperations: %v", err)
	}
	op, err := reloaded.BrowserBatchOperationGet("op-1")
	if err != nil {
		t.Fatalf("BrowserBatchOperationGet: %v", err)
	}
	if op.Status != "interrupted" {
		t.Fatalf("status = %q", op.Status)
	}
	if len(op.Results) != 2 || !op.Results[0].Completed || op.Results[1].Completed {
		t.Fatalf("per-item completion state was not preserved: %#v", op.Results)
	}
	if op.FinishedAt == "" {
		t.Fatal("interrupted operation must receive finishedAt")
	}
}
