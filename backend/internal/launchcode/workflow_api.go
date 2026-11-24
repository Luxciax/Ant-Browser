package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"ant-chrome/backend/internal/automation"
)

type WorkflowProvider interface {
	BrowserAutomationWorkflowList() ([]automation.WorkflowDefinition, error)
	BrowserAutomationWorkflowGet(id string) (*automation.WorkflowDefinition, error)
	BrowserAutomationWorkflowSave(input automation.WorkflowDefinition) (*automation.WorkflowDefinition, error)
	BrowserAutomationWorkflowDelete(id string) error
	BrowserAutomationWorkflowRun(req automation.WorkflowRunRequest) (*automation.WorkflowRunRecord, error)
	BrowserAutomationWorkflowResume(runID string, force bool) (*automation.WorkflowRunRecord, error)
	BrowserAutomationWorkflowRunList(limit int) ([]automation.WorkflowRunRecord, error)
	BrowserAutomationWorkflowRunGet(id string) (*automation.WorkflowRunRecord, error)
}

func (s *LaunchServer) workflowProvider() (WorkflowProvider, error) {
	p, ok := s.starter.(WorkflowProvider)
	if !ok { return nil, newServiceError(http.StatusServiceUnavailable, "workflow api is unavailable") }
	return p, nil
}
func (s *LaunchServer) ListWorkflows() ([]automation.WorkflowDefinition,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowList()}
func (s *LaunchServer) GetWorkflow(id string)(*automation.WorkflowDefinition,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowGet(id)}
func (s *LaunchServer) SaveWorkflow(v automation.WorkflowDefinition)(*automation.WorkflowDefinition,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowSave(v)}
func (s *LaunchServer) DeleteWorkflow(id string)error{p,e:=s.workflowProvider();if e!=nil{return e};return p.BrowserAutomationWorkflowDelete(id)}
func (s *LaunchServer) RunWorkflow(req automation.WorkflowRunRequest)(*automation.WorkflowRunRecord,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowRun(req)}
func (s *LaunchServer) ResumeWorkflow(id string,force bool)(*automation.WorkflowRunRecord,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowResume(id,force)}
func (s *LaunchServer) ListWorkflowRuns(limit int)([]automation.WorkflowRunRecord,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowRunList(limit)}
func (s *LaunchServer) GetWorkflowRun(id string)(*automation.WorkflowRunRecord,error){p,e:=s.workflowProvider();if e!=nil{return nil,e};return p.BrowserAutomationWorkflowRunGet(id)}

func decodeJSONBody(r *http.Request, dst any) error {
	dec:=json.NewDecoder(io.LimitReader(r.Body,2<<20));dec.DisallowUnknownFields();return dec.Decode(dst)
}
func (s *LaunchServer) handleWorkflows(w http.ResponseWriter,r *http.Request){
	switch r.Method{
	case http.MethodGet:
		items,err:=s.ListWorkflows();if err!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":err.Error()});return}
		writeJSON(w,200,map[string]any{"ok":true,"items":items,"count":len(items)})
	case http.MethodPost:
		var input automation.WorkflowDefinition;if err:=decodeJSONBody(r,&input);err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":"invalid request body"});return}
		item,err:=s.SaveWorkflow(input);if err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":err.Error()});return}
		writeJSON(w,200,map[string]any{"ok":true,"item":item})
	default: writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"})
	}
}
func (s *LaunchServer) handleWorkflowByID(w http.ResponseWriter,r *http.Request){
	id:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/automation/workflows/"),"/")
	if id==""||strings.Contains(id,"/"){writeJSON(w,404,map[string]any{"ok":false,"error":"workflow not found"});return}
	switch r.Method{
	case http.MethodGet:
		item,err:=s.GetWorkflow(id);if err!=nil{writeJSON(w,404,map[string]any{"ok":false,"error":err.Error()});return};writeJSON(w,200,map[string]any{"ok":true,"item":item})
	case http.MethodDelete:
		if err:=s.DeleteWorkflow(id);err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":err.Error()});return};writeJSON(w,200,map[string]any{"ok":true})
	default: writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"})
	}
}
func (s *LaunchServer) handleWorkflowRun(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	var req automation.WorkflowRunRequest;if err:=decodeJSONBody(r,&req);err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":"invalid request body"});return}
	item,err:=s.RunWorkflow(req);if err!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":err.Error(),"run":item});return};writeJSON(w,200,map[string]any{"ok":true,"run":item})
}
func (s *LaunchServer) handleWorkflowRuns(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	limit:=50;if raw:=strings.TrimSpace(r.URL.Query().Get("limit"));raw!=""{if n,e:=strconv.Atoi(raw);e==nil{limit=n}}
	items,err:=s.ListWorkflowRuns(limit);if err!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":err.Error()});return};writeJSON(w,200,map[string]any{"ok":true,"items":items,"count":len(items)})
}
func (s *LaunchServer) handleWorkflowRunByID(w http.ResponseWriter,r *http.Request){
	path:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/automation/workflow-runs/"),"/")
	parts:=strings.Split(path,"/");if len(parts)<1||parts[0]==""{writeJSON(w,404,map[string]any{"ok":false,"error":"run not found"});return}
	if len(parts)==2&&parts[1]=="resume"{
		if r.Method!=http.MethodPost{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
		var body struct{Force bool `json:"force"`};_ = decodeJSONBody(r,&body)
		item,err:=s.ResumeWorkflow(parts[0],body.Force);if err!=nil{writeJSON(w,409,map[string]any{"ok":false,"error":err.Error(),"run":item});return};writeJSON(w,200,map[string]any{"ok":true,"run":item});return
	}
	if len(parts)!=1||r.Method!=http.MethodGet{writeJSON(w,404,map[string]any{"ok":false,"error":"run not found"});return}
	item,err:=s.GetWorkflowRun(parts[0]);if err!=nil{writeJSON(w,404,map[string]any{"ok":false,"error":err.Error()});return};writeJSON(w,200,map[string]any{"ok":true,"run":item})
}
