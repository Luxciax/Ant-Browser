package backend

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type BrowserRuntimeEvidenceV2 struct {
	SchemaVersion       int                           `json:"schemaVersion"`
	ProfileID           string                        `json:"profileId"`
	EnvironmentID       string                        `json:"environmentId,omitempty"`
	EnvironmentRevision string                        `json:"environmentRevision,omitempty"`
	PlanDigest          string                        `json:"planDigest,omitempty"`
	ObservedAt          string                        `json:"observedAt"`
	ExecutionStatus     string                        `json:"executionStatus"`
	ObservationStatus   string                        `json:"observationStatus"`
	AssertionStatus     string                        `json:"assertionStatus"`
	Fields              []BrowserRuntimeEvidenceField   `json:"fields"`
	Contexts            []BrowserRuntimeContextEvidence `json:"contexts"`
	Issues              []string                      `json:"issues,omitempty"`
	Limitations         []string                      `json:"limitations,omitempty"`
}

type BrowserRuntimeEvidenceField struct {
	Key        string   `json:"key"`
	Capability string   `json:"capability"`
	Contexts   []string `json:"contexts,omitempty"`
	Observed   bool     `json:"observed"`
	Assertion  string   `json:"assertion"`
	Note       string   `json:"note,omitempty"`
}

type BrowserRuntimeContextEvidence struct {
	Context    string `json:"context"`
	Applicable bool   `json:"applicable"`
	Executed   bool   `json:"executed"`
	Error      string `json:"error,omitempty"`
}


func runtimeEvidenceAssertion(expected, actual string, observed bool) string {
	if !observed {
		return "unobserved"
	}
	if strings.TrimSpace(expected) == "" {
		return "not_asserted"
	}
	if expected == actual {
		return "passed"
	}
	return "mismatch"
}

func platformExpectedRuntime(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows":
		return "Win32"
	case "linux":
		return "Linux x86_64"
	case "mac", "macos":
		return "MacIntel"
	default:
		return ""
	}
}

func buildRuntimeEvidenceV2(check *BrowserFingerprintCheckResult) BrowserRuntimeEvidenceV2 {
	evidence := BrowserRuntimeEvidenceV2{
		SchemaVersion: 2,
		ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		ExecutionStatus: "failed",
		ObservationStatus: "unavailable",
		AssertionStatus: "not_run",
		Fields: []BrowserRuntimeEvidenceField{},
		Contexts: []BrowserRuntimeContextEvidence{},
	}
	if check == nil {
		evidence.Issues = []string{"runtime check returned no result"}
		return evidence
	}
	evidence.ProfileID = check.ProfileId
	evidence.ExecutionStatus = "success"
	evidence.Issues = append([]string{}, check.Issues...)
	evidence.Limitations = append([]string{}, check.Limitations...)

	add := func(key, capability, expected, actual string, observed bool, contexts []string, note string) {
		evidence.Fields = append(evidence.Fields, BrowserRuntimeEvidenceField{
			Key: key, Capability: capability, Contexts: append([]string{}, contexts...),
			Observed: observed, Assertion: runtimeEvidenceAssertion(expected, actual, observed), Note: note,
		})
	}
	r := check.Runtime
	e := check.Expected
	contextStatus := func(name string, ctx *BrowserFingerprintWorkerInfo) {
		if ctx == nil {
			evidence.Contexts = append(evidence.Contexts, BrowserRuntimeContextEvidence{Context: name})
			return
		}
		evidence.Contexts = append(evidence.Contexts, BrowserRuntimeContextEvidence{Context: name, Applicable: ctx.Applicable, Executed: ctx.Executed, Error: ctx.Error})
	}
	contextStatus("dedicated-worker", r.Worker)
	contextStatus("iframe", r.Iframe)
	contextStatus("shared-worker", r.SharedWorker)
	evidence.Contexts = append(evidence.Contexts, BrowserRuntimeContextEvidence{Context: "service-worker", Applicable: r.ServiceWorker.Applicable, Executed: r.ServiceWorker.Executed, Error: r.ServiceWorker.Error})
	add("language", "语言", e.Language, r.Language, strings.TrimSpace(r.Language) != "", []string{"window"}, "")
	add("timezone", "时区", e.Timezone, r.Timezone, strings.TrimSpace(r.Timezone) != "", []string{"window"}, "")
	hardwareActual := ""
	if r.HardwareConcurrency > 0 {
		hardwareActual = strconv.Itoa(r.HardwareConcurrency)
	}
	add("hardwareConcurrency", "CPU 核心数", e.HardwareConcurrency, hardwareActual, hardwareActual != "", []string{"window"}, "")
	platformExpected := platformExpectedRuntime(e.Platform)
	add("platform", "平台", platformExpected, r.Platform, strings.TrimSpace(r.Platform) != "", []string{"window"}, "")

	add("canvas", "Canvas ImageData", "", r.CanvasHash, strings.TrimSpace(r.CanvasHash) != "", []string{"window"}, "hash observed; no expected hash is asserted")
	add("audio", "Audio", "", r.AudioHash, strings.TrimSpace(r.AudioHash) != "", []string{"window"}, "hash observed; no expected hash is asserted")
	add("clientRects", "ClientRects", "", r.ClientRectsHash, strings.TrimSpace(r.ClientRectsHash) != "", []string{"window"}, "hash observed; no expected hash is asserted")
	add("webglVendor", "WebGL Vendor", e.WebGLVendor, r.WebGLVendor, strings.TrimSpace(r.WebGLVendor) != "", []string{"window"}, "")
	add("webglRenderer", "WebGL Renderer", e.WebGLRenderer, r.WebGLRenderer, strings.TrimSpace(r.WebGLRenderer) != "", []string{"window"}, "")
	add("userAgent", "User Agent", "", r.UserAgent, strings.TrimSpace(r.UserAgent) != "", []string{"window"}, "observed only")
	add("userAgentData", "UA-CH", "", r.UserAgentData, strings.TrimSpace(r.UserAgentData) != "", []string{"window"}, "observed only")

	addContextFields := func(prefix string, contexts []string, ctx *BrowserFingerprintWorkerInfo) {
		if ctx == nil || !ctx.Applicable || !ctx.Executed || ctx.Error != "" {
			return
		}
		ctxHC := ""
		if ctx.HardwareConcurrency > 0 {
			ctxHC = strconv.Itoa(ctx.HardwareConcurrency)
		}
		add(prefix+".language", "跨上下文语言", r.Language, ctx.Language, strings.TrimSpace(ctx.Language) != "", contexts, "")
		add(prefix+".timezone", "跨上下文时区", r.Timezone, ctx.Timezone, strings.TrimSpace(ctx.Timezone) != "", contexts, "")
		add(prefix+".hardwareConcurrency", "跨上下文 CPU", hardwareActual, ctxHC, ctxHC != "", contexts, "")
		add(prefix+".platform", "跨上下文平台", r.Platform, ctx.Platform, strings.TrimSpace(ctx.Platform) != "", contexts, "")
		add(prefix+".userAgent", "跨上下文 User Agent", r.UserAgent, ctx.UserAgent, strings.TrimSpace(ctx.UserAgent) != "", contexts, "")
	}
	addContextFields("worker", []string{"window", "dedicated-worker"}, r.Worker)
	addContextFields("iframe", []string{"window", "iframe"}, r.Iframe)
	addContextFields("sharedWorker", []string{"window", "shared-worker"}, r.SharedWorker)

	observed := 0
	for _, field := range evidence.Fields {
		if field.Observed {
			observed++
		}
	}
	if observed > 0 {
		evidence.ObservationStatus = "observed"
	}
	switch check.Status {
	case "passed_basic":
		evidence.AssertionStatus = "passed_basic"
	case "mismatch":
		evidence.AssertionStatus = "mismatch"
	case "unverified":
		evidence.AssertionStatus = "unverified"
	default:
		evidence.AssertionStatus = "unknown"
	}
	return evidence
}

func (a *App) attachRuntimeEvidenceContext(evidence *BrowserRuntimeEvidenceV2) {
	if a == nil || evidence == nil || strings.TrimSpace(evidence.ProfileID) == "" {
		return
	}
	if a.browserMgr != nil {
		if contract, err := a.browserMgr.EnvironmentContract(evidence.ProfileID); err == nil && contract != nil {
			evidence.EnvironmentID = contract.Environment.EnvironmentID
			evidence.EnvironmentRevision = contract.Environment.Revision
		}
	}
	if receipt, err := a.BrowserProfileLaunchReceipt(evidence.ProfileID); err == nil && receipt != nil {
		if evidence.EnvironmentRevision == "" || receipt.Plan.EnvironmentRevision == evidence.EnvironmentRevision {
			evidence.PlanDigest = receipt.Plan.Digest
		}
	}
}

func (a *App) storeRuntimeEvidence(evidence BrowserRuntimeEvidenceV2) {
	if a == nil || strings.TrimSpace(evidence.ProfileID) == "" {
		return
	}
	a.runtimeEvidenceMu.Lock()
	if a.runtimeEvidence == nil {
		a.runtimeEvidence = make(map[string]BrowserRuntimeEvidenceV2)
	}
	a.runtimeEvidence[evidence.ProfileID] = evidence
	a.runtimeEvidenceMu.Unlock()
}

func cloneRuntimeEvidence(evidence BrowserRuntimeEvidenceV2) BrowserRuntimeEvidenceV2 {
	out := evidence
	out.Issues = append([]string{}, evidence.Issues...)
	out.Limitations = append([]string{}, evidence.Limitations...)
	out.Fields = append([]BrowserRuntimeEvidenceField{}, evidence.Fields...)
	out.Contexts = append([]BrowserRuntimeContextEvidence{}, evidence.Contexts...)
	for index := range out.Fields {
		out.Fields[index].Contexts = append([]string{}, evidence.Fields[index].Contexts...)
	}
	return out
}

func (a *App) BrowserProfileRuntimeEvidence(profileID string) (*BrowserRuntimeEvidenceV2, error) {
	if a == nil {
		return nil, fmt.Errorf("application is unavailable")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, fmt.Errorf("profile id is required")
	}
	a.runtimeEvidenceMu.RLock()
	evidence, ok := a.runtimeEvidence[profileID]
	a.runtimeEvidenceMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("runtime evidence not found")
	}
	copyEvidence := cloneRuntimeEvidence(evidence)
	return &copyEvidence, nil
}

// BrowserProfileRuntimeVerify is explicit: ordinary runtime/status reads never
// invoke the CDP fingerprint check.
func (a *App) BrowserProfileRuntimeVerify(profileID string) (*BrowserRuntimeEvidenceV2, error) {
	check, err := a.BrowserProfileFingerprintCheck(profileID)
	if err != nil {
		return nil, err
	}
	evidence := buildRuntimeEvidenceV2(check)
	a.attachRuntimeEvidenceContext(&evidence)
	a.storeRuntimeEvidence(evidence)
	copyEvidence := cloneRuntimeEvidence(evidence)
	return &copyEvidence, nil
}

func evidenceFieldByCapability(evidence *BrowserRuntimeEvidenceV2, capability string) (BrowserRuntimeEvidenceField, bool) {
	if evidence == nil {
		return BrowserRuntimeEvidenceField{}, false
	}
	for _, field := range evidence.Fields {
		if strings.EqualFold(strings.TrimSpace(field.Capability), strings.TrimSpace(capability)) {
			return field, true
		}
	}
	return BrowserRuntimeEvidenceField{}, false
}
