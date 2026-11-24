package mcpserver

import (
	"context"
	"fmt"

	"ant-chrome/backend/internal/browser"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type phase4Provider interface {
	EnvironmentView(profileID string) (map[string]interface{}, error)
	Diagnostics() (map[string]interface{}, error)
	BatchOperation(operationID string) (map[string]interface{}, error)
	BatchCreate(inputs []browser.ProfileInput, clientKeys []string, concurrency int) (map[string]interface{}, error)
	BatchStart(profileIDs []string, concurrency int) (map[string]interface{}, error)
	BatchStop(profileIDs []string, concurrency int) (map[string]interface{}, error)
}

type batchCreateItemInput struct {
	ClientKey string `json:"clientKey,omitempty"`
	Config browser.ProfileInput `json:"config"`
}
type batchCreateInput struct {
	Items []batchCreateItemInput `json:"items"`
	Concurrency int `json:"concurrency,omitempty"`
}

type batchProfilesInput struct {
	ProfileIDs []string `json:"profileIds"`
	Concurrency int `json:"concurrency,omitempty"`
}
type batchOperationInput struct { OperationID string `json:"operationId"` }

func registerPhase4Tools(srv *mcp.Server, p InstanceProvider) {
	provider, ok := p.(phase4Provider)
	if !ok { return }
	if providerAllowsScope(p, "environment-read") {
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_diagnostics",Description:"读取脱敏控制面诊断、批量任务计数和 SecretStore 就绪状态。",Annotations:readOnly("读取诊断")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,phase2Output,error){
			data,err:=provider.Diagnostics(); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_batch_get",Description:"按 operationId 查询批量操作结果，用于部分失败后的恢复判断。",Annotations:readOnly("查询批量操作")},func(_ context.Context,_ *mcp.CallToolRequest,in batchOperationInput)(*mcp.CallToolResult,phase2Output,error){
			if in.OperationID==""{return toolError(fmt.Errorf("operationId is required")),phase2Output{},nil}; data,err:=provider.BatchOperation(in.OperationID); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_view",Description:"读取 UI/自动化共享的 EnvironmentView v2 聚合状态。",Annotations:readOnly("读取环境聚合视图")},func(_ context.Context,_ *mcp.CallToolRequest,in phase2ReadInput)(*mcp.CallToolResult,phase2Output,error){
			profileID,err:=resolvePhase2Profile(p,in.Selector); if err!=nil{return toolError(err),phase2Output{},nil}; data,err:=provider.EnvironmentView(profileID); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
	}
	if providerAllowsScope(p, "environment-admin") {
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_batch_create",Description:"批量创建实例，返回逐项结果和 operationId。",Annotations:mutating("批量创建实例")},func(_ context.Context,_ *mcp.CallToolRequest,in batchCreateInput)(*mcp.CallToolResult,phase2Output,error){
			inputs:=make([]browser.ProfileInput,0,len(in.Items)); keys:=make([]string,0,len(in.Items))
			for _,item:=range in.Items{inputs=append(inputs,item.Config);keys=append(keys,item.ClientKey)}
			data,err:=provider.BatchCreate(inputs,keys,in.Concurrency); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
	}
	if providerAllowsScope(p, "page-action") {
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_batch_start",Description:"按 profileId 批量启动实例，返回逐项结果和 operationId。",Annotations:mutating("批量启动实例")},func(_ context.Context,_ *mcp.CallToolRequest,in batchProfilesInput)(*mcp.CallToolResult,phase2Output,error){
			data,err:=provider.BatchStart(in.ProfileIDs,in.Concurrency); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_batch_stop",Description:"按 profileId 批量停止实例，返回逐项结果和 operationId。",Annotations:destructive("批量停止实例")},func(_ context.Context,_ *mcp.CallToolRequest,in batchProfilesInput)(*mcp.CallToolResult,phase2Output,error){
			data,err:=provider.BatchStop(in.ProfileIDs,in.Concurrency); if err!=nil{return toolError(err),phase2Output{},nil}; return nil,phase2Output{Data:data},nil
		})
	}
}
