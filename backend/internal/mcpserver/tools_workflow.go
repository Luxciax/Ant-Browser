package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/automation"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type workflowIDInput struct{ ID string `json:"id"` }
type workflowRunIDInput struct{ ID string `json:"id"`; Force bool `json:"force,omitempty"` }
type workflowListOutput struct{ Items []automation.WorkflowDefinition `json:"items"`; Count int `json:"count"` }
type workflowRunsOutput struct{ Items []automation.WorkflowRunRecord `json:"items"`; Count int `json:"count"` }

func registerWorkflowTools(srv *mcp.Server,p WorkflowProvider){
	if providerAllowsScope(p,"environment-read"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_list",Description:"列出可视化工作流。",Annotations:readOnly("列出工作流")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,workflowListOutput,error){
			items,e:=p.ListWorkflows();if e!=nil{return toolError(e),workflowListOutput{},nil};return nil,workflowListOutput{Items:items,Count:len(items)},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_runs",Description:"列出最近工作流运行记录。",Annotations:readOnly("工作流历史")},func(_ context.Context,_ *mcp.CallToolRequest,in struct{Limit int `json:"limit,omitempty"`})(*mcp.CallToolResult,workflowRunsOutput,error){
			items,e:=p.ListWorkflowRuns(in.Limit);if e!=nil{return toolError(e),workflowRunsOutput{},nil};return nil,workflowRunsOutput{Items:items,Count:len(items)},nil
		})
	}
	if providerAllowsScope(p,"environment-admin"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_save",Description:"创建或更新工作流定义。",Annotations:mutating("保存工作流")},func(_ context.Context,_ *mcp.CallToolRequest,in automation.WorkflowDefinition)(*mcp.CallToolResult,automation.WorkflowDefinition,error){
			v,e:=p.SaveWorkflow(in);if e!=nil{return toolError(e),automation.WorkflowDefinition{},nil};return nil,*v,nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_delete",Description:"删除工作流定义，不删除历史运行记录。",Annotations:destructive("删除工作流")},func(_ context.Context,_ *mcp.CallToolRequest,in workflowIDInput)(*mcp.CallToolResult,map[string]any,error){
			if strings.TrimSpace(in.ID)==""{return toolError(fmt.Errorf("id is required")),nil,nil};if e:=p.DeleteWorkflow(in.ID);e!=nil{return toolError(e),nil,nil};return nil,map[string]any{"deleted":true},nil
		})
	}
	if providerAllowsScope(p,"script"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_run",Description:"执行工作流并持久化步骤历史。",Annotations:mutating("运行工作流")},func(_ context.Context,_ *mcp.CallToolRequest,in automation.WorkflowRunRequest)(*mcp.CallToolResult,automation.WorkflowRunRecord,error){
			v,e:=p.RunWorkflow(in);if e!=nil{return toolError(e),valueOrEmptyWorkflowRun(v),nil};return nil,*v,nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_workflow_resume",Description:"恢复 paused/interrupted/failed 工作流；副作用断点需 force=true。",Annotations:mutating("恢复工作流")},func(_ context.Context,_ *mcp.CallToolRequest,in workflowRunIDInput)(*mcp.CallToolResult,automation.WorkflowRunRecord,error){
			v,e:=p.ResumeWorkflow(in.ID,in.Force);if e!=nil{return toolError(e),valueOrEmptyWorkflowRun(v),nil};return nil,*v,nil
		})
	}
}
func valueOrEmptyWorkflowRun(v *automation.WorkflowRunRecord) automation.WorkflowRunRecord{if v==nil{return automation.WorkflowRunRecord{}};return *v}
