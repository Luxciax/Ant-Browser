package mcpserver

import (
	"context"
	"strings"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Selector struct {
	Code        string   `json:"code,omitempty" jsonschema:"快捷启动码，最精确的定位方式"`
	ProfileID   string   `json:"profileId,omitempty" jsonschema:"实例的稳定 ID"`
	ProfileName string   `json:"profileName,omitempty" jsonschema:"实例名称，需完全匹配"`
	Keywords    []string `json:"keywords,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	GroupID     string   `json:"groupId,omitempty"`
	MatchMode   string   `json:"matchMode,omitempty" jsonschema:"unique 或 first"`
}

func (s Selector) toLaunchSelector() launchcode.LaunchSelector {
	return launchcode.LaunchSelector{Code: strings.TrimSpace(s.Code), ProfileID: strings.TrimSpace(s.ProfileID), ProfileName: strings.TrimSpace(s.ProfileName), Keywords: s.Keywords, Tags: s.Tags, GroupID: strings.TrimSpace(s.GroupID), MatchMode: strings.TrimSpace(s.MatchMode)}
}

type InstanceSummary struct {
	ProfileID string `json:"profileId"`
	ProfileName string `json:"profileName"`
	LaunchCode string `json:"launchCode"`
	GroupID string `json:"groupId,omitempty"`
	Tags []string `json:"tags,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	ProxyID string `json:"proxyId,omitempty"`
	CoreID string `json:"coreId,omitempty"`
	Running bool `json:"running"`
	DebugReady bool `json:"debugReady"`
}

func toSummary(p *browser.Profile) InstanceSummary {
	if p == nil { return InstanceSummary{} }
	return InstanceSummary{ProfileID:p.ProfileId, ProfileName:p.ProfileName, LaunchCode:p.LaunchCode, GroupID:p.GroupId, Tags:p.Tags, Keywords:p.Keywords, ProxyID:p.ProxyId, CoreID:p.CoreId, Running:p.Running, DebugReady:p.DebugReady}
}

func toSummaries(items []browser.Profile) []InstanceSummary {
	out := make([]InstanceSummary, 0, len(items))
	for i := range items { out = append(out, toSummary(&items[i])) }
	return out
}

type RuntimeState struct {
	ProfileID string `json:"profileId"`
	ProfileName string `json:"profileName"`
	LaunchCode string `json:"launchCode,omitempty"`
	Running bool `json:"running"`
	DebugReady bool `json:"debugReady"`
	Pid int `json:"pid,omitempty"`
	DebugPort int `json:"debugPort,omitempty"`
	RuntimeWarning string `json:"runtimeWarning,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

func toRuntimeState(p *browser.Profile) RuntimeState {
	if p == nil { return RuntimeState{} }
	return RuntimeState{ProfileID:p.ProfileId, ProfileName:p.ProfileName, LaunchCode:p.LaunchCode, Running:p.Running, DebugReady:p.DebugReady, Pid:p.Pid, DebugPort:p.DebugPort, RuntimeWarning:p.RuntimeWarning, LastError:p.LastError}
}

func registerInstanceTools(srv *mcp.Server, p InstanceProvider) {
	registerInstanceQueryTools(srv,p); registerInstanceWriteTools(srv,p); registerRuntimeTools(srv,p)
}

type listInstancesInput struct { Tag string `json:"tag,omitempty"`; GroupID string `json:"groupId,omitempty"`; Keyword string `json:"keyword,omitempty"`; RunningOnly bool `json:"runningOnly,omitempty"` }
type listInstancesOutput struct { Count int `json:"count"`; Items []InstanceSummary `json:"items"` }
type getInstanceInput struct { Selector Selector `json:"selector"` }
type getInstanceOutput struct { Instance InstanceSummary `json:"instance"`; Profile *browser.Profile `json:"profile"` }

func registerInstanceQueryTools(srv *mcp.Server, p InstanceProvider) {
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_list",Description:"列出浏览器实例，可按标签、分组、关键字和运行状态过滤。",Annotations:readOnly("列出实例")},func(_ context.Context,_ *mcp.CallToolRequest,in listInstancesInput)(*mcp.CallToolResult,listInstancesOutput,error){
		items,err:=p.ListProfiles(); if err!=nil{return toolError(err),listInstancesOutput{},nil}; filtered:=filterInstances(items,in); return nil,listInstancesOutput{Count:len(filtered),Items:toSummaries(filtered)},nil
	})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_get",Description:"按 selector 查询单个实例的完整配置。",Annotations:readOnly("查询实例详情")},func(_ context.Context,_ *mcp.CallToolRequest,in getInstanceInput)(*mcp.CallToolResult,getInstanceOutput,error){
		profile,err:=p.FindProfile(in.Selector.toLaunchSelector()); if err!=nil{return toolError(err),getInstanceOutput{},nil}; return nil,getInstanceOutput{Instance:toSummary(profile),Profile:profile},nil
	})
}

func filterInstances(items []browser.Profile,in listInstancesInput) []browser.Profile {
	tag:=strings.TrimSpace(in.Tag); groupID:=strings.TrimSpace(in.GroupID); keyword:=strings.ToLower(strings.TrimSpace(in.Keyword)); out:=make([]browser.Profile,0,len(items))
	for _,item:=range items { if in.RunningOnly&&!item.Running{continue}; if groupID!=""&&strings.TrimSpace(item.GroupId)!=groupID{continue}; if tag!=""&&!containsFold(item.Tags,tag){continue}; if keyword!=""&&!matchesKeyword(item,keyword){continue}; out=append(out,item) }; return out
}
func containsFold(values []string,target string) bool { for _,v:=range values{if strings.EqualFold(strings.TrimSpace(v),target){return true}}; return false }
func matchesKeyword(item browser.Profile,keyword string) bool { if strings.Contains(strings.ToLower(item.ProfileName),keyword)||strings.Contains(strings.ToLower(item.LaunchCode),keyword){return true}; for _,v:=range item.Keywords{if strings.Contains(strings.ToLower(v),keyword){return true}}; return false }

type InstanceConfig struct { ProfileName string `json:"profileName"`; UserDataDir string `json:"userDataDir,omitempty"`; CoreID string `json:"coreId,omitempty"`; ProxyID string `json:"proxyId,omitempty"`; GroupID string `json:"groupId,omitempty"`; Tags []string `json:"tags,omitempty"`; Keywords []string `json:"keywords,omitempty"`; MemoryLimitMB int `json:"memoryLimitMb,omitempty"` }
func (c InstanceConfig) toProfileInput() browser.ProfileInput { return browser.ProfileInput{ProfileName:strings.TrimSpace(c.ProfileName),UserDataDir:strings.TrimSpace(c.UserDataDir),CoreId:strings.TrimSpace(c.CoreID),ProxyId:strings.TrimSpace(c.ProxyID),GroupId:strings.TrimSpace(c.GroupID),Tags:c.Tags,Keywords:c.Keywords,MemoryLimitMB:c.MemoryLimitMB} }
type createInstanceInput struct { Config InstanceConfig `json:"config"`; LaunchCode string `json:"launchCode,omitempty"` }
type createInstanceOutput struct { Instance InstanceSummary `json:"instance"` }
type updateInstanceInput struct { Selector Selector `json:"selector"`; Config InstanceConfig `json:"config"`; LaunchCode string `json:"launchCode,omitempty"` }
type deleteInstanceInput struct { Selector Selector `json:"selector"` }
type deleteInstanceOutput struct { Deleted bool `json:"deleted"`; ProfileID string `json:"profileId"`; ProfileName string `json:"profileName"` }

func registerInstanceWriteTools(srv *mcp.Server,p InstanceProvider){
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_create",Description:"创建新的浏览器实例。",Annotations:mutating("创建实例")},func(_ context.Context,_ *mcp.CallToolRequest,in createInstanceInput)(*mcp.CallToolResult,createInstanceOutput,error){profile,code,err:=p.CreateProfile(in.Config.toProfileInput(),strings.TrimSpace(in.LaunchCode)); if err!=nil{return toolError(err),createInstanceOutput{},nil}; s:=toSummary(profile); s.LaunchCode=code; return nil,createInstanceOutput{Instance:s},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_update",Description:"整体更新实例配置；建议先读取当前配置。",Annotations:mutating("更新实例")},func(_ context.Context,_ *mcp.CallToolRequest,in updateInstanceInput)(*mcp.CallToolResult,createInstanceOutput,error){target,err:=p.FindProfile(in.Selector.toLaunchSelector()); if err!=nil{return toolError(err),createInstanceOutput{},nil}; profile,code,err:=p.UpdateProfile(target.ProfileId,in.Config.toProfileInput(),strings.TrimSpace(in.LaunchCode)); if err!=nil{return toolError(err),createInstanceOutput{},nil}; s:=toSummary(profile); s.LaunchCode=code; return nil,createInstanceOutput{Instance:s},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_delete",Description:"删除实例；运行中的实例需先停止。",Annotations:destructive("删除实例")},func(_ context.Context,_ *mcp.CallToolRequest,in deleteInstanceInput)(*mcp.CallToolResult,deleteInstanceOutput,error){target,err:=p.FindProfile(in.Selector.toLaunchSelector()); if err!=nil{return toolError(err),deleteInstanceOutput{},nil}; if err:=p.DeleteProfile(target.ProfileId);err!=nil{return toolError(err),deleteInstanceOutput{},nil}; return nil,deleteInstanceOutput{Deleted:true,ProfileID:target.ProfileId,ProfileName:target.ProfileName},nil})
}

type startInstanceInput struct { Selector Selector `json:"selector"`; StartURLs []string `json:"startUrls,omitempty"`; SkipDefaultStartURLs bool `json:"skipDefaultStartUrls,omitempty"`; ProxyID string `json:"proxyId,omitempty"` }
type startInstanceOutput struct { Runtime RuntimeState `json:"runtime"` }
type runtimeSessionInput struct { Selector Selector `json:"selector"`; StartURLs []string `json:"startUrls,omitempty"`; SkipDefaultStartURLs bool `json:"skipDefaultStartUrls,omitempty"`; TimeoutMs int `json:"timeoutMs,omitempty"` }
type runtimeSessionOutput struct { Ready bool `json:"ready"`; CDPURL string `json:"cdpUrl,omitempty"`; Runtime RuntimeState `json:"runtime"`; Hint string `json:"hint,omitempty"` }
type stopInstanceInput struct { Selector Selector `json:"selector"` }
type stopInstanceOutput struct { Stopped bool `json:"stopped"`; Runtime RuntimeState `json:"runtime"` }
type runtimeStatusInput struct { Selector Selector `json:"selector"` }
type runtimeStatusOutput struct { Runtime RuntimeState `json:"runtime"` }
type activeSessionOutput struct { Active bool `json:"active"`; CDPURL string `json:"cdpUrl,omitempty"`; Runtime RuntimeState `json:"runtime,omitzero"` }

func registerRuntimeTools(srv *mcp.Server,p InstanceProvider){
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_start",Description:"启动实例但不等待调试端口就绪。",Annotations:mutating("启动实例")},func(_ context.Context,_ *mcp.CallToolRequest,in startInstanceInput)(*mcp.CallToolResult,startInstanceOutput,error){profile,code,err:=p.StartProfile(in.Selector.toLaunchSelector(),launchcode.LaunchRequestParams{StartURLs:in.StartURLs,SkipDefaultStartURLs:in.SkipDefaultStartURLs,ProxyId:strings.TrimSpace(in.ProxyID)}); if err!=nil{return toolError(err),startInstanceOutput{},nil}; s:=toRuntimeState(profile); if s.LaunchCode==""{s.LaunchCode=code}; return nil,startInstanceOutput{Runtime:s},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_runtime_session",Description:"启动实例并等待 CDP 可接管，返回统一 CDP 入口。",Annotations:mutating("接管实例会话")},func(_ context.Context,_ *mcp.CallToolRequest,in runtimeSessionInput)(*mcp.CallToolResult,runtimeSessionOutput,error){session,err:=p.OpenRuntimeSession(in.Selector.toLaunchSelector(),launchcode.LaunchRequestParams{StartURLs:in.StartURLs,SkipDefaultStartURLs:in.SkipDefaultStartURLs},time.Duration(in.TimeoutMs)*time.Millisecond); if err!=nil{return toolError(err),runtimeSessionOutput{},nil}; out:=runtimeSessionOutput{Ready:session.Ready,CDPURL:session.CDPURL,Runtime:toRuntimeState(session.Profile)}; if !session.Ready{out.Hint="浏览器已启动但调试端口尚未就绪，请稍后重试。"}; return nil,out,nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_runtime_status",Description:"查询实例当前运行态，不触发启动。",Annotations:readOnly("查询运行态")},func(_ context.Context,_ *mcp.CallToolRequest,in runtimeStatusInput)(*mcp.CallToolResult,runtimeStatusOutput,error){target,err:=p.FindProfile(in.Selector.toLaunchSelector()); if err!=nil{return toolError(err),runtimeStatusOutput{},nil}; profile,err:=p.StatusProfile(target.ProfileId); if err!=nil{return toolError(err),runtimeStatusOutput{},nil}; return nil,runtimeStatusOutput{Runtime:toRuntimeState(profile)},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_runtime_active",Description:"查询当前挂在统一 CDP 入口上的实例。",Annotations:readOnly("查询活动会话")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,activeSessionOutput,error){session,err:=p.ActiveRuntimeSession(); if err!=nil{return toolError(err),activeSessionOutput{},nil}; if session==nil{return nil,activeSessionOutput{Active:false},nil}; return nil,activeSessionOutput{Active:true,CDPURL:session.CDPURL,Runtime:toRuntimeState(session.Profile)},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_instance_stop",Description:"停止运行中的真实浏览器实例。",Annotations:destructive("停止实例")},func(_ context.Context,_ *mcp.CallToolRequest,in stopInstanceInput)(*mcp.CallToolResult,stopInstanceOutput,error){target,err:=p.FindProfile(in.Selector.toLaunchSelector()); if err!=nil{return toolError(err),stopInstanceOutput{},nil}; profile,err:=p.StopProfile(target.ProfileId); if err!=nil{return toolError(err),stopInstanceOutput{},nil}; return nil,stopInstanceOutput{Stopped:true,Runtime:toRuntimeState(profile)},nil})
}
