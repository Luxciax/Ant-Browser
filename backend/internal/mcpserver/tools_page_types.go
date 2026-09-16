package mcpserver

import (
	"strings"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PageState struct { URL string `json:"url,omitempty"`; Title string `json:"title,omitempty"` }
type pageOutput struct { ProfileID string `json:"profileId"`; OK bool `json:"ok"`; Page PageState `json:"page"`; Error string `json:"error,omitempty"` }

func pageCall(p PageProvider, selector *Selector, timeoutMs int, action string, args map[string]any) (*launchcode.PageResult, error) {
	req := launchcode.PageRequest{Steps: []launchcode.PageStep{{Action: action, Args: args}}, TimeoutMs: timeoutMs}
	if selector != nil { req.Selector = selector.toLaunchSelector() }
	return p.RunPageSteps(req)
}

func firstStep(result *launchcode.PageResult) (map[string]any, error) {
	if result == nil { return nil, &launchcode.ServiceError{Status:500,Message:"页面操作没有返回结果"} }
	if len(result.Steps)==0 { msg:=strings.TrimSpace(result.Error); if msg==""{msg="页面操作没有返回结果"}; return nil,&launchcode.ServiceError{Status:500,Message:msg} }
	step:=result.Steps[0]; if !step.OK { msg:=strings.TrimSpace(step.Error); if msg==""{msg="页面操作失败"}; return step.Result,&launchcode.ServiceError{Status:500,Message:msg} }
	return step.Result,nil
}
func pageStateOf(payload map[string]any) PageState { if payload==nil{return PageState{}}; u,_:=payload["url"].(string); t,_:=payload["title"].(string); return PageState{URL:u,Title:t} }
func runPageAction(p PageProvider, selector *Selector, timeoutMs int, action string, args map[string]any)(*mcp.CallToolResult,pageOutput,map[string]any){
	result,err:=pageCall(p,selector,timeoutMs,action,args); if err!=nil{return toolError(err),pageOutput{},nil}; payload,stepErr:=firstStep(result); if stepErr!=nil{return toolError(stepErr),pageOutput{ProfileID:result.ProfileID,Error:stepErr.Error()},payload}; return nil,pageOutput{ProfileID:result.ProfileID,OK:true,Page:pageStateOf(payload)},payload
}

type targetInput struct { Selector *Selector `json:"selector,omitempty"`; TimeoutMs int `json:"timeoutMs,omitempty"` }
type elementInput struct { targetInput; Element string `json:"element"`; FrameSelector string `json:"frameSelector,omitempty"`; Index *int `json:"index,omitempty"` }
func (in elementInput) args(extra map[string]any) map[string]any { args:=map[string]any{"selector":strings.TrimSpace(in.Element)}; if f:=strings.TrimSpace(in.FrameSelector);f!=""{args["frameSelector"]=f}; if in.Index!=nil{args["index"]=*in.Index}; for k,v:=range extra{args[k]=v}; return args }
type gotoInput struct { targetInput; URL string `json:"url"`; WaitUntil string `json:"waitUntil,omitempty"` }
type gotoOutput struct { pageOutput; Status int `json:"status,omitempty"` }
type snapshotInput struct { targetInput; Limit int `json:"limit,omitempty"`; IncludeText bool `json:"includeText,omitempty"` }
type PageElement struct { Role string `json:"role"`; Tag string `json:"tag,omitempty"`; Name string `json:"name,omitempty"`; Selector string `json:"selector"`; Type string `json:"type,omitempty"`; Value string `json:"value,omitempty"`; Href string `json:"href,omitempty"`; Checked *bool `json:"checked,omitempty"`; Disabled bool `json:"disabled,omitempty"` }
type snapshotOutput struct { pageOutput; Elements []PageElement `json:"elements"`; Text string `json:"text,omitempty"`; Truncated bool `json:"truncated,omitempty"` }
type screenshotInput struct { targetInput; Element string `json:"element,omitempty"`; FullPage bool `json:"fullPage,omitempty"`; Type string `json:"type,omitempty"`; Quality int `json:"quality,omitempty"` }
type clickInput struct { elementInput; Button string `json:"button,omitempty"`; ClickCount int `json:"clickCount,omitempty"`; Force bool `json:"force,omitempty"` }
type FillField struct { Element string `json:"element"`; Value string `json:"value"`; FrameSelector string `json:"frameSelector,omitempty"` }
type fillInput struct { targetInput; Fields []FillField `json:"fields"` }
type fillOutput struct { pageOutput; Filled []string `json:"filled"` }
type pressInput struct { targetInput; Input string `json:"key"`; Element string `json:"element,omitempty"`; FrameSelector string `json:"frameSelector,omitempty"` }
type selectInput struct { elementInput; Values []string `json:"values"` }
type selectOutput struct { pageOutput; Selected []string `json:"selected"` }
type waitInput struct { targetInput; Element string `json:"element,omitempty"`; State string `json:"state,omitempty"`; URL string `json:"url,omitempty"`; LoadState string `json:"loadState,omitempty"` }
type extractInput struct { elementInput; Mode string `json:"mode,omitempty"`; Attribute string `json:"attribute,omitempty"`; All bool `json:"all,omitempty"` }
type extractOutput struct { ProfileID string `json:"profileId"`; OK bool `json:"ok"`; Mode string `json:"mode"`; Value string `json:"value,omitempty"`; Values []string `json:"values,omitempty"`; Count int `json:"count,omitempty"` }
type evaluateInput struct { targetInput; Expression string `json:"expression"`; Arg any `json:"arg,omitempty"` }
type evaluateOutput struct { ProfileID string `json:"profileId"`; OK bool `json:"ok"`; Value any `json:"value"` }
type tabsInput struct { targetInput; Op string `json:"op,omitempty"`; Index *int `json:"index,omitempty"`; URL string `json:"url,omitempty"` }
type PageTab struct { Index int `json:"index"`; URL string `json:"url,omitempty"`; Title string `json:"title,omitempty"`; Active bool `json:"active,omitempty"` }
type tabsOutput struct { ProfileID string `json:"profileId"`; OK bool `json:"ok"`; Count int `json:"count"`; Items []PageTab `json:"items,omitempty"`; Page PageState `json:"page,omitempty"` }
type releaseInput struct { Selector *Selector `json:"selector,omitempty"` }
type releaseOutput struct { ProfileID string `json:"profileId"`; Released bool `json:"released"` }
