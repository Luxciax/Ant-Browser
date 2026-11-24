package automation

type WorkflowSelector struct {
	Code        string   `json:"code,omitempty"`
	ProfileID   string   `json:"profileId,omitempty"`
	ProfileName string   `json:"profileName,omitempty"`
	GroupID     string   `json:"groupId,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	MatchMode   string   `json:"matchMode,omitempty"`
}

type WorkflowRetryPolicy struct {
	MaxAttempts      int  `json:"maxAttempts,omitempty"`
	DelayMs          int  `json:"delayMs,omitempty"`
	AllowSideEffects bool `json:"allowSideEffects,omitempty"`
}

type WorkflowCondition struct {
	Variable string `json:"variable"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type WorkflowStep struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	Action        string              `json:"action,omitempty"`
	Args          map[string]any      `json:"args,omitempty"`
	SaveAs        string              `json:"saveAs,omitempty"`
	Value         any                 `json:"value,omitempty"`
	Condition     *WorkflowCondition  `json:"condition,omitempty"`
	Steps         []WorkflowStep      `json:"steps,omitempty"`
	ElseSteps     []WorkflowStep      `json:"elseSteps,omitempty"`
	MaxIterations int                 `json:"maxIterations,omitempty"`
	Retry         WorkflowRetryPolicy `json:"retry,omitempty"`
	TimeoutMs     int                 `json:"timeoutMs,omitempty"`
	Breakpoint    bool                `json:"breakpoint,omitempty"`
}

type WorkflowSchedule struct {
	Enabled      bool   `json:"enabled"`
	EveryMinutes int    `json:"everyMinutes,omitempty"`
	DailyTime    string `json:"dailyTime,omitempty"`
	AllowOverlap bool   `json:"allowOverlap,omitempty"`
}

type WorkflowDefinition struct {
	SchemaVersion int              `json:"schemaVersion"`
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Enabled       bool             `json:"enabled"`
	Selector      WorkflowSelector `json:"selector"`
	Variables     map[string]any   `json:"variables,omitempty"`
	Steps         []WorkflowStep   `json:"steps"`
	Schedule      WorkflowSchedule `json:"schedule"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
	LastRunAt     string           `json:"lastRunAt,omitempty"`
}

type WorkflowRunStepRecord struct {
	StepID     string         `json:"stepId"`
	StepType   string         `json:"stepType"`
	Action     string         `json:"action,omitempty"`
	Index      int            `json:"index"`
	Attempt    int            `json:"attempt"`
	OK         bool           `json:"ok"`
	Result     map[string]any `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	StartedAt  string         `json:"startedAt"`
	FinishedAt string         `json:"finishedAt"`
}

type WorkflowRunRecord struct {
	SchemaVersion       int                     `json:"schemaVersion"`
	ID                  string                  `json:"id"`
	WorkflowID          string                  `json:"workflowId"`
	WorkflowName        string                  `json:"workflowName"`
	Status              string                  `json:"status"`
	Selector            WorkflowSelector        `json:"selector"`
	Variables           map[string]any          `json:"variables"`
	NextStep            int                     `json:"nextStep"`
	LastStepID          string                  `json:"lastStepId,omitempty"`
	ResumeRequiresForce bool                    `json:"resumeRequiresForce,omitempty"`
	Steps               []WorkflowRunStepRecord `json:"steps"`
	Error               string                  `json:"error,omitempty"`
	StartedAt           string                  `json:"startedAt"`
	FinishedAt          string                  `json:"finishedAt,omitempty"`
	UpdatedAt           string                  `json:"updatedAt"`
}

type WorkflowRunRequest struct {
	WorkflowID  string         `json:"workflowId"`
	Variables   map[string]any `json:"variables,omitempty"`
	Breakpoints []string       `json:"breakpoints,omitempty"`
}
