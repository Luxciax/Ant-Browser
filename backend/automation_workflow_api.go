package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/launchcode"
)

type BrowserAutomationWorkflow = automation.WorkflowDefinition
type BrowserAutomationWorkflowRun = automation.WorkflowRunRecord
type BrowserAutomationWorkflowRunRequest = automation.WorkflowRunRequest

func (a *App) workflowStoreOrError() (*automation.WorkflowStore, error) {
	if a == nil || a.workflowStore == nil { return nil, fmt.Errorf("workflow store is unavailable") }
	return a.workflowStore, nil
}

func (a *App) BrowserAutomationWorkflowList() ([]BrowserAutomationWorkflow, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	return store.ListDefinitions()
}

func (a *App) BrowserAutomationWorkflowGet(id string) (*BrowserAutomationWorkflow, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	value, err := store.GetDefinition(id); if err != nil { return nil, err }
	return &value, nil
}

func (a *App) BrowserAutomationWorkflowSave(input BrowserAutomationWorkflow) (*BrowserAutomationWorkflow, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	if err := validateWorkflow(input); err != nil { return nil, err }
	value, err := store.SaveDefinition(input); if err != nil { return nil, err }
	return &value, nil
}

func (a *App) BrowserAutomationWorkflowDelete(id string) error {
	store, err := a.workflowStoreOrError(); if err != nil { return err }
	return store.DeleteDefinition(id)
}

func (a *App) BrowserAutomationWorkflowRunList(limit int) ([]BrowserAutomationWorkflowRun, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	return store.ListRuns(limit)
}

func (a *App) BrowserAutomationWorkflowRunGet(id string) (*BrowserAutomationWorkflowRun, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	value, err := store.GetRun(id); if err != nil { return nil, err }
	return &value, nil
}

func validateWorkflow(workflow automation.WorkflowDefinition) error {
	if strings.TrimSpace(workflow.Name) == "" { return fmt.Errorf("workflow name is required") }
	if len(workflow.Steps) == 0 { return fmt.Errorf("workflow requires at least one step") }
	seen := map[string]struct{}{}
	var walk func([]automation.WorkflowStep) error
	walk = func(steps []automation.WorkflowStep) error {
		for index, step := range steps {
			if strings.TrimSpace(step.ID) == "" { return fmt.Errorf("step %d is missing id", index+1) }
			if _, ok := seen[step.ID]; ok { return fmt.Errorf("duplicate workflow step id %q", step.ID) }
			seen[step.ID] = struct{}{}
			switch strings.ToLower(strings.TrimSpace(step.Type)) {
			case "action":
				if !workflowActionAllowed(step.Action) { return fmt.Errorf("step %s uses unsupported action %q", step.ID, step.Action) }
				attempts := step.Retry.MaxAttempts
				if attempts == 0 { attempts = 1 }
				if attempts < 1 || attempts > 5 { return fmt.Errorf("step %s retry maxAttempts must be 1..5", step.ID) }
				if isWorkflowSideEffectAction(step.Action) && attempts > 1 && !step.Retry.AllowSideEffects {
					return fmt.Errorf("step %s retries a side-effect action without allowSideEffects", step.ID)
				}
			case "set", "sleep":
			case "if", "loop":
				if step.Condition == nil && strings.EqualFold(step.Type, "if") { return fmt.Errorf("step %s requires condition", step.ID) }
				if strings.EqualFold(step.Type, "loop") && (step.MaxIterations < 1 || step.MaxIterations > 100) {
					return fmt.Errorf("step %s maxIterations must be 1..100", step.ID)
				}
				if err := walk(step.Steps); err != nil { return err }
				if err := walk(step.ElseSteps); err != nil { return err }
			default:
				return fmt.Errorf("step %s uses unsupported type %q", step.ID, step.Type)
			}
		}
		return nil
	}
	return walk(workflow.Steps)
}

func workflowActionAllowed(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "goto", "snapshot", "click", "fill", "press", "selectoption", "waitfor", "extract", "evaluate", "screenshot", "tabs", "upload":
		return true
	default:
		return false
	}
}

func isWorkflowSideEffectAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "click", "fill", "press", "selectoption", "evaluate", "tabs", "upload":
		return true
	default:
		return false
	}
}

func workflowSelector(value automation.WorkflowSelector) launchcode.LaunchSelector {
	return launchcode.LaunchSelector{
		Code: value.Code, ProfileID: value.ProfileID, ProfileName: value.ProfileName,
		GroupID: value.GroupID, Keywords: value.Keywords, Tags: value.Tags, MatchMode: value.MatchMode,
	}
}

func cloneWorkflowVariables(source map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range source { out[key] = value }
	return out
}

func workflowLookup(variables map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	var current any = variables
	for _, part := range parts {
		object, ok := current.(map[string]any); if !ok { return nil, false }
		current, ok = object[part]; if !ok { return nil, false }
	}
	return current, true
}

func workflowString(value any) string {
	if text, ok := value.(string); ok { return text }
	data, _ := json.Marshal(value)
	return string(data)
}

func interpolateWorkflowValue(value any, variables map[string]any) any {
	switch typed := value.(type) {
	case string:
		out := typed
		for {
			start := strings.Index(out, "{{"); if start < 0 { break }
			endRelative := strings.Index(out[start+2:], "}}"); if endRelative < 0 { break }
			end := start + 2 + endRelative
			key := strings.TrimSpace(out[start+2 : end])
			replacement, ok := workflowLookup(variables, key); if !ok { replacement = "" }
			if start == 0 && end+2 == len(out) { return replacement }
			out = out[:start] + workflowString(replacement) + out[end+2:]
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed { out[key] = interpolateWorkflowValue(child, variables) }
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed { out[index] = interpolateWorkflowValue(child, variables) }
		return out
	default:
		return value
	}
}

func workflowConditionOK(condition *automation.WorkflowCondition, variables map[string]any) bool {
	if condition == nil { return false }
	actual, exists := workflowLookup(variables, condition.Variable)
	operator := strings.ToLower(strings.TrimSpace(condition.Operator)); if operator == "" { operator = "eq" }
	expected := interpolateWorkflowValue(condition.Value, variables)
	switch operator {
	case "exists": return exists
	case "not-exists": return !exists
	case "eq": return workflowString(actual) == workflowString(expected)
	case "ne": return workflowString(actual) != workflowString(expected)
	case "contains": return strings.Contains(workflowString(actual), workflowString(expected))
	case "truthy":
		if !exists || actual == nil { return false }
		if value, ok := actual.(bool); ok { return value }
		text := workflowString(actual)
		return text != "" && text != "0" && text != "false"
	default:
		return false
	}
}

func (a *App) BrowserAutomationWorkflowRun(req BrowserAutomationWorkflowRunRequest) (*BrowserAutomationWorkflowRun, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	definition, err := store.GetDefinition(req.WorkflowID); if err != nil { return nil, err }
	if err := validateWorkflow(definition); err != nil { return nil, err }
	now := time.Now().UTC().Format(time.RFC3339Nano)
	run := automation.WorkflowRunRecord{
		WorkflowID: definition.ID, WorkflowName: definition.Name, Status: "running", Selector: definition.Selector,
		Variables: cloneWorkflowVariables(definition.Variables), NextStep: 0, StartedAt: now, UpdatedAt: now,
	}
	for key, value := range req.Variables { run.Variables[key] = value }
	run, err = store.SaveRun(run); if err != nil { return nil, err }
	return a.executeWorkflowRun(definition, run, req.Breakpoints)
}

func (a *App) BrowserAutomationWorkflowResume(runID string, force bool) (*BrowserAutomationWorkflowRun, error) {
	store, err := a.workflowStoreOrError(); if err != nil { return nil, err }
	run, err := store.GetRun(runID); if err != nil { return nil, err }
	if run.Status != "paused" && run.Status != "interrupted" && run.Status != "failed" {
		return nil, fmt.Errorf("workflow run is not resumable")
	}
	if run.ResumeRequiresForce && !force {
		return nil, fmt.Errorf("resume requires force because the pending step may have side effects")
	}
	definition, err := store.GetDefinition(run.WorkflowID); if err != nil { return nil, err }
	run.Status = "running"; run.Error = ""; run.FinishedAt = ""; run.ResumeRequiresForce = false
	run, err = store.SaveRun(run); if err != nil { return nil, err }
	return a.executeWorkflowRun(definition, run, nil)
}

func (a *App) executeWorkflowRun(definition automation.WorkflowDefinition, run automation.WorkflowRunRecord, breakpoints []string) (*automation.WorkflowRunRecord, error) {
	store, _ := a.workflowStoreOrError()
	breakpointSet := map[string]struct{}{}
	for _, value := range breakpoints { breakpointSet[strings.TrimSpace(value)] = struct{}{} }
	for index := run.NextStep; index < len(definition.Steps); index++ {
		step := definition.Steps[index]
		_, explicitBreakpoint := breakpointSet[step.ID]
		if explicitBreakpoint || step.Breakpoint {
			run.Status = "paused"; run.NextStep = index; run.LastStepID = step.ID
			run.ResumeRequiresForce = isWorkflowStepPotentiallySideEffecting(step)
			saved, _ := store.SaveRun(run)
			return &saved, nil
		}
		run.LastStepID = step.ID; run.NextStep = index
		run.ResumeRequiresForce = isWorkflowStepPotentiallySideEffecting(step)
		run, _ = store.SaveRun(run)
		if err := a.executeWorkflowStep(definition.Selector, &run, step, index); err != nil {
			run.Status = "failed"; run.Error = err.Error(); run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			saved, _ := store.SaveRun(run)
			return &saved, err
		}
		run.NextStep = index + 1; run.ResumeRequiresForce = false
		run, _ = store.SaveRun(run)
	}
	run.Status = "success"; run.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	saved, err := store.SaveRun(run); if err != nil { return nil, err }
	return &saved, nil
}

func isWorkflowStepPotentiallySideEffecting(step automation.WorkflowStep) bool {
	if strings.EqualFold(step.Type, "action") { return isWorkflowSideEffectAction(step.Action) }
	children := append(append([]automation.WorkflowStep{}, step.Steps...), step.ElseSteps...)
	for _, child := range children { if isWorkflowStepPotentiallySideEffecting(child) { return true } }
	return false
}

func (a *App) executeWorkflowStep(selector automation.WorkflowSelector, run *automation.WorkflowRunRecord, step automation.WorkflowStep, index int) error {
	switch strings.ToLower(strings.TrimSpace(step.Type)) {
	case "set":
		if strings.TrimSpace(step.SaveAs) == "" { return fmt.Errorf("step %s requires saveAs", step.ID) }
		run.Variables[step.SaveAs] = interpolateWorkflowValue(step.Value, run.Variables)
		return nil
	case "sleep":
		milliseconds := step.TimeoutMs
		if milliseconds <= 0 {
			if number, ok := step.Value.(float64); ok { milliseconds = int(number) }
		}
		if milliseconds < 0 || milliseconds > 300000 { return fmt.Errorf("step %s sleep is out of range", step.ID) }
		time.Sleep(time.Duration(milliseconds) * time.Millisecond)
		return nil
	case "if":
		branch := step.ElseSteps
		if workflowConditionOK(step.Condition, run.Variables) { branch = step.Steps }
		for childIndex, child := range branch {
			if err := a.executeWorkflowStep(selector, run, child, childIndex); err != nil { return err }
		}
		return nil
	case "loop":
		for iteration := 0; iteration < step.MaxIterations; iteration++ {
			run.Variables["loop"] = map[string]any{"index": iteration}
			if step.Condition != nil && !workflowConditionOK(step.Condition, run.Variables) { break }
			for childIndex, child := range step.Steps {
				if err := a.executeWorkflowStep(selector, run, child, childIndex); err != nil { return err }
			}
		}
		return nil
	case "action":
		return a.executeWorkflowAction(selector, run, step, index)
	default:
		return fmt.Errorf("unsupported workflow step type %q", step.Type)
	}
}

func (a *App) executeWorkflowAction(selector automation.WorkflowSelector, run *automation.WorkflowRunRecord, step automation.WorkflowStep, index int) error {
	attempts := step.Retry.MaxAttempts; if attempts <= 0 { attempts = 1 }
	delay := time.Duration(step.Retry.DelayMs) * time.Millisecond
	timeout := step.TimeoutMs; if timeout <= 0 { timeout = 30000 }; if timeout > 300000 { timeout = 300000 }
	argsRaw := interpolateWorkflowValue(step.Args, run.Variables)
	args, _ := argsRaw.(map[string]any)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		started := time.Now().UTC()
		result, err := a.launchServer.RunPageSteps(launchcode.PageRequest{
			Selector: workflowSelector(selector), TimeoutMs: timeout,
			Steps: []launchcode.PageStep{{Action: step.Action, Args: args}},
		})
		record := automation.WorkflowRunStepRecord{
			StepID: step.ID, StepType: step.Type, Action: step.Action, Index: index, Attempt: attempt,
			StartedAt: started.Format(time.RFC3339Nano), FinishedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
		if err == nil && result != nil && result.OK && len(result.Steps) > 0 && result.Steps[0].OK {
			record.OK = true; record.Result = result.Steps[0].Result; run.Steps = append(run.Steps, record)
			if strings.TrimSpace(step.SaveAs) != "" { run.Variables[step.SaveAs] = record.Result }
			return nil
		}
		switch {
		case err != nil: lastErr = err
		case result != nil && strings.TrimSpace(result.Error) != "": lastErr = fmt.Errorf("%s", strings.TrimSpace(result.Error))
		default: lastErr = fmt.Errorf("page action failed")
		}
		record.Error = lastErr.Error(); run.Steps = append(run.Steps, record)
		if attempt < attempts && delay > 0 { time.Sleep(delay) }
	}
	return fmt.Errorf("step %s failed after %d attempt(s): %w", step.ID, attempts, lastErr)
}

func (a *App) BrowserAutomationWorkflowValidate(input BrowserAutomationWorkflow) map[string]any {
	if err := validateWorkflow(input); err != nil { return map[string]any{"valid": false, "error": err.Error()} }
	return map[string]any{"valid": true, "stepCount": len(input.Steps)}
}

func parseWorkflowDailyTime(value string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":"); if len(parts) != 2 { return 0, 0, false }
	hour, errHour := strconv.Atoi(parts[0]); minute, errMinute := strconv.Atoi(parts[1])
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 { return 0, 0, false }
	return hour, minute, true
}

func workflowScheduleDue(workflow automation.WorkflowDefinition, now time.Time) bool {
	if !workflow.Enabled || !workflow.Schedule.Enabled { return false }
	last, _ := time.Parse(time.RFC3339Nano, workflow.LastRunAt)
	if workflow.Schedule.EveryMinutes > 0 {
		return last.IsZero() || now.Sub(last) >= time.Duration(workflow.Schedule.EveryMinutes)*time.Minute
	}
	if hour, minute, ok := parseWorkflowDailyTime(workflow.Schedule.DailyTime); ok {
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		return !now.Before(target) && (last.IsZero() || last.Before(target))
	}
	return false
}

func (a *App) startWorkflowScheduler() {
	if a == nil || a.workflowStore == nil { return }
	a.workflowSchedulerMu.Lock()
	if a.workflowSchedulerCancel != nil { a.workflowSchedulerMu.Unlock(); return }
	ctx, cancel := context.WithCancel(context.Background())
	a.workflowSchedulerCancel = cancel
	a.workflowSchedulerMu.Unlock()
	go func() {
		ticker := time.NewTicker(30 * time.Second); defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case now := <-ticker.C: a.runDueWorkflows(now)
			}
		}
	}()
}

func (a *App) stopWorkflowScheduler() {
	a.workflowSchedulerMu.Lock()
	cancel := a.workflowSchedulerCancel; a.workflowSchedulerCancel = nil
	a.workflowSchedulerMu.Unlock()
	if cancel != nil { cancel() }
}

func (a *App) runDueWorkflows(now time.Time) {
	items, err := a.workflowStore.ListDefinitions(); if err != nil { return }
	for _, workflow := range items {
		if !workflowScheduleDue(workflow, now) { continue }
		a.workflowActiveMu.Lock()
		active := a.workflowActive[workflow.ID]
		if active && !workflow.Schedule.AllowOverlap { a.workflowActiveMu.Unlock(); continue }
		a.workflowActive[workflow.ID] = true
		a.workflowActiveMu.Unlock()
		workflow.LastRunAt = now.UTC().Format(time.RFC3339Nano)
		_, _ = a.workflowStore.SaveDefinition(workflow)
		go func(id string) {
			defer func() {
				a.workflowActiveMu.Lock()
				delete(a.workflowActive, id)
				a.workflowActiveMu.Unlock()
			}()
			_, _ = a.BrowserAutomationWorkflowRun(automation.WorkflowRunRequest{WorkflowID: id})
		}(workflow.ID)
	}
}
