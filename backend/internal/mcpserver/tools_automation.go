package mcpserver

import (
	"context"
	"encoding/json"
	"strings"

	"ant-chrome/backend/internal/automation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ScriptSummary struct { ID string `json:"id"`; Name string `json:"name"`; Description string `json:"description,omitempty"`; Type string `json:"type"`; Status string `json:"status"`; Tags []string `json:"tags,omitempty"` }
func toScriptSummary(r automation.ScriptRecord) ScriptSummary { return ScriptSummary{ID:r.ID,Name:r.Name,Description:r.Description,Type:r.Type,Status:r.Status,Tags:r.Tags} }
type ScriptDetail struct { ScriptSummary; Notes string `json:"notes,omitempty"`; EntryFile string `json:"entryFile,omitempty"`; TargetMode string `json:"targetMode,omitempty"`; DefaultParams map[string]any `json:"defaultParams,omitempty"`; DefaultSelector map[string]any `json:"defaultSelector,omitempty"`; CreatedAt string `json:"createdAt,omitempty"`; UpdatedAt string `json:"updatedAt,omitempty"` }
func decodeJSONObject(text string) map[string]any { trimmed:=strings.TrimSpace(text); if trimmed==""{return nil}; var out map[string]any; if json.Unmarshal([]byte(trimmed),&out)!=nil{return nil}; return out }
type ScriptRun struct { ID string `json:"id"`; ScriptID string `json:"scriptId"`; ScriptName string `json:"scriptName,omitempty"`; Status string `json:"status"`; Summary string `json:"summary,omitempty"`; Error string `json:"error,omitempty"`; ResultText string `json:"resultText,omitempty"`; StartedAt string `json:"startedAt,omitempty"`; DurationMs int64 `json:"durationMs,omitempty"` }
func toScriptRun(r automation.ScriptRunRecord) ScriptRun { return ScriptRun{ID:r.ID,ScriptID:r.ScriptID,ScriptName:r.ScriptName,Status:r.Status,Summary:r.Summary,Error:r.Error,ResultText:r.ResultText,StartedAt:r.StartedAt,DurationMs:r.DurationMs} }

type listScriptsOutput struct { Count int `json:"count"`; Items []ScriptSummary `json:"items"` }
type getScriptInput struct { ScriptID string `json:"scriptId"` }
type getScriptOutput struct { Script ScriptDetail `json:"script"` }
type runScriptInput struct { ScriptID string `json:"scriptId"`; Selector *Selector `json:"selector,omitempty"`; Params map[string]any `json:"params,omitempty"`; TimeoutMs int `json:"timeoutMs,omitempty"` }
type runScriptOutput struct { Run ScriptRun `json:"run"` }
type listScriptRunsInput struct { Limit int `json:"limit,omitempty"` }
type listScriptRunsOutput struct { Count int `json:"count"`; Items []ScriptRun `json:"items"` }

func registerAutomationTools(srv *mcp.Server,p AutomationProvider){
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_script_list",Description:"列出已导入的自动化脚本。",Annotations:readOnly("列出自动化脚本")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,listScriptsOutput,error){items,err:=p.ListScripts(); if err!=nil{return toolError(err),listScriptsOutput{},nil}; out:=make([]ScriptSummary,0,len(items)); for _,item:=range items{out=append(out,toScriptSummary(item))}; return nil,listScriptsOutput{Count:len(out),Items:out},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_script_get",Description:"查询脚本详情及默认目标、默认参数。",Annotations:readOnly("查询脚本详情")},func(_ context.Context,_ *mcp.CallToolRequest,in getScriptInput)(*mcp.CallToolResult,getScriptOutput,error){record,err:=p.GetScript(in.ScriptID); if err!=nil{return toolError(err),getScriptOutput{},nil}; d:=ScriptDetail{ScriptSummary:toScriptSummary(*record),Notes:record.Notes,EntryFile:record.EntryFile,TargetMode:record.TargetConfig.Mode,DefaultParams:decodeJSONObject(record.ParamsText),DefaultSelector:decodeJSONObject(record.SelectorText),CreatedAt:record.CreatedAt,UpdatedAt:record.UpdatedAt}; return nil,getScriptOutput{Script:d},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_script_run",Description:"执行自动化脚本并等待结果。",Annotations:mutating("执行自动化脚本")},func(_ context.Context,_ *mcp.CallToolRequest,in runScriptInput)(*mcp.CallToolResult,runScriptOutput,error){req,err:=buildScriptRunRequest(in); if err!=nil{return toolError(err),runScriptOutput{},nil}; record,err:=p.RunScript(req); if err!=nil{return toolError(err),runScriptOutput{},nil}; run:=toScriptRun(*record); if strings.EqualFold(run.Status,"failed"){return toolError(errorFromRun(run)),runScriptOutput{Run:run},nil}; return nil,runScriptOutput{Run:run},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_script_runs",Description:"查询最近的脚本执行记录。",Annotations:readOnly("查询脚本执行记录")},func(_ context.Context,_ *mcp.CallToolRequest,in listScriptRunsInput)(*mcp.CallToolResult,listScriptRunsOutput,error){items,err:=p.ListScriptRuns(in.Limit); if err!=nil{return toolError(err),listScriptRunsOutput{},nil}; out:=make([]ScriptRun,0,len(items)); for _,item:=range items{out=append(out,toScriptRun(item))}; return nil,listScriptRunsOutput{Count:len(out),Items:out},nil})
}

func buildScriptRunRequest(in runScriptInput)(automation.ScriptRunRequest,error){
	req:=automation.ScriptRunRequest{ScriptID:strings.TrimSpace(in.ScriptID),TimeoutMs:in.TimeoutMs}
	if in.Selector==nil{req.UseScriptSelector=true}else{encoded,err:=json.Marshal(in.Selector); if err!=nil{return req,newInputError("selector 无法序列化："+err.Error())}; req.SelectorText=string(encoded)}
	if len(in.Params)==0{req.UseScriptParams=true}else{encoded,err:=json.Marshal(in.Params); if err!=nil{return req,newInputError("params 无法序列化："+err.Error())}; req.ParamsText=string(encoded)}
	return req,nil
}
