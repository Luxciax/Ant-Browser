package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type phase5ControlProvider interface {
	TrashList() (map[string]interface{}, error)
	TrashRestore(id string) (map[string]interface{}, error)
	TrashPermanentlyDelete(id string) error
	TrashCleanup() error
	SyncStatus() (map[string]interface{}, error)
}
type trashInput struct{ ProfileID string `json:"profileId"` }

func registerPhase5ControlTools(srv *mcp.Server,p Provider){
	provider,ok:=any(p).(phase5ControlProvider);if !ok{return}
	if providerAllowsScope(p,"environment-read"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_trash_list",Description:"列出软删除实例；不暴露代理凭据或启动参数。",Annotations:readOnly("列出回收站")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,phase2Output,error){
			v,e:=provider.TrashList();if e!=nil{return toolError(e),phase2Output{},nil};return nil,phase2Output{Data:v},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_sync_status",Description:"读取备份通道同步状态；不会触发同步。",Annotations:readOnly("读取同步状态")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,phase2Output,error){
			v,e:=provider.SyncStatus();if e!=nil{return toolError(e),phase2Output{},nil};return nil,phase2Output{Data:v},nil
		})
	}
	if providerAllowsScope(p,"environment-admin"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_trash_restore",Description:"从回收站恢复实例。",Annotations:mutating("恢复实例")},func(_ context.Context,_ *mcp.CallToolRequest,in trashInput)(*mcp.CallToolResult,phase2Output,error){
			if strings.TrimSpace(in.ProfileID)==""{return toolError(fmt.Errorf("profileId is required")),phase2Output{},nil};v,e:=provider.TrashRestore(in.ProfileID);if e!=nil{return toolError(e),phase2Output{},nil};return nil,phase2Output{Data:v},nil
		})
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_trash_permanent_delete",Description:"永久删除回收站实例及其受管关联数据。",Annotations:destructive("永久删除实例")},func(_ context.Context,_ *mcp.CallToolRequest,in trashInput)(*mcp.CallToolResult,map[string]any,error){
			if strings.TrimSpace(in.ProfileID)==""{return toolError(fmt.Errorf("profileId is required")),nil,nil};if e:=provider.TrashPermanentlyDelete(in.ProfileID);e!=nil{return toolError(e),nil,nil};return nil,map[string]any{"deleted":true},nil
		})
	}
}
