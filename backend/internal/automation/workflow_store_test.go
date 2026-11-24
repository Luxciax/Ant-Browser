package automation

import (
	"testing"
)

func TestWorkflowStoreRecoversRunningRunsAsInterrupted(t *testing.T) {
	store := NewWorkflowStore(t.TempDir())
	run, err := store.SaveRun(WorkflowRunRecord{
		WorkflowID: "workflow-1",
		WorkflowName: "recover",
		Status: "running",
		NextStep: 2,
		ResumeRequiresForce: true,
	})
	if err != nil {
		t.Fatalf("SaveRun: %v", err)
	}
	if err := store.RecoverInterruptedRuns(); err != nil {
		t.Fatalf("RecoverInterruptedRuns: %v", err)
	}
	recovered, err := store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if recovered.Status != "interrupted" {
		t.Fatalf("status = %q", recovered.Status)
	}
	if recovered.NextStep != 2 || !recovered.ResumeRequiresForce {
		t.Fatalf("resume metadata was not preserved: %#v", recovered)
	}
	if recovered.FinishedAt == "" {
		t.Fatal("interrupted run must receive a finish timestamp")
	}
}
