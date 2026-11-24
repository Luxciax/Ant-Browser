package backend

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/automation"
)

func TestValidateWorkflowRejectsImplicitSideEffectRetry(t *testing.T) {
	workflow := automation.WorkflowDefinition{
		Name: "retry-guard",
		Steps: []automation.WorkflowStep{{
			ID: "click-1",
			Type: "action",
			Action: "click",
			Retry: automation.WorkflowRetryPolicy{MaxAttempts: 2},
		}},
	}
	err := validateWorkflow(workflow)
	if err == nil || !strings.Contains(err.Error(), "allowSideEffects") {
		t.Fatalf("expected side-effect retry validation error, got %v", err)
	}

	workflow.Steps[0].Retry.AllowSideEffects = true
	if err := validateWorkflow(workflow); err != nil {
		t.Fatalf("explicit side-effect retry should be accepted: %v", err)
	}
}

func TestValidateWorkflowRejectsUnboundedLoopAndDuplicateIDs(t *testing.T) {
	workflow := automation.WorkflowDefinition{
		Name: "loop-guard",
		Steps: []automation.WorkflowStep{{
			ID: "loop",
			Type: "loop",
			MaxIterations: 101,
		}},
	}
	if err := validateWorkflow(workflow); err == nil {
		t.Fatal("expected maxIterations validation error")
	}

	workflow.Steps = []automation.WorkflowStep{
		{ID: "same", Type: "sleep", TimeoutMs: 10},
		{ID: "same", Type: "set", SaveAs: "value"},
	}
	if err := validateWorkflow(workflow); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate step id error, got %v", err)
	}
}

func TestWorkflowInterpolationPreservesWholeValueType(t *testing.T) {
	vars := map[string]any{
		"count": float64(3),
		"nested": map[string]any{"name": "Ant"},
	}
	if got := interpolateWorkflowValue("{{count}}", vars); got != float64(3) {
		t.Fatalf("whole variable should preserve type, got %#v", got)
	}
	if got := interpolateWorkflowValue("hello {{nested.name}}", vars); got != "hello Ant" {
		t.Fatalf("unexpected interpolated value: %#v", got)
	}
}
