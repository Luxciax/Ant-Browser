package mcpserver

import (
	"context"

	"ant-chrome/backend/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ProxyNode struct { ProxyID string `json:"proxyId"`; ProxyName string `json:"proxyName"`; GroupName string `json:"groupName,omitempty"`; Protocol string `json:"protocol,omitempty"`; LatencyMs int64 `json:"latencyMs,omitempty"`; LastTestOk bool `json:"lastTestOk"`; LastTestedAt string `json:"lastTestedAt,omitempty"` }
func detectProtocol(proxyConfig string) string { for i:=0;i<len(proxyConfig);i++{if proxyConfig[i]==':'{return proxyConfig[:i]}; c:=proxyConfig[i]; if !((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='+'||c=='-'||c=='.'){return ""}}; return "" }
func toProxyNode(p config.BrowserProxy) ProxyNode { return ProxyNode{ProxyID:p.ProxyId,ProxyName:p.ProxyName,GroupName:p.GroupName,Protocol:detectProtocol(p.ProxyConfig),LatencyMs:p.LastLatencyMs,LastTestOk:p.LastTestOk,LastTestedAt:p.LastTestedAt} }
type BrowserCoreInfo struct { CoreID string `json:"coreId"`; CoreName string `json:"coreName"`; IsDefault bool `json:"isDefault"` }
type listProxiesInput struct { GroupName string `json:"groupName,omitempty"`; AvailableOnly bool `json:"availableOnly,omitempty"` }
type listProxiesOutput struct { Count int `json:"count"`; Items []ProxyNode `json:"items"` }
type proxyIDInput struct { ProxyID string `json:"proxyId"` }
type testProxySpeedOutput struct { ProxyID string `json:"proxyId"`; Ok bool `json:"ok"`; LatencyMs int64 `json:"latencyMs"`; Engine string `json:"engine,omitempty"`; Error string `json:"error,omitempty"` }
type checkProxyHealthOutput struct { ProxyID string `json:"proxyId"`; Ok bool `json:"ok"`; IP string `json:"ip,omitempty"`; Country string `json:"country,omitempty"`; Region string `json:"region,omitempty"`; City string `json:"city,omitempty"`; AsOrganization string `json:"asOrganization,omitempty"`; FraudScore int64 `json:"fraudScore,omitempty"`; IsResidential bool `json:"isResidential"`; Source string `json:"source,omitempty"`; Error string `json:"error,omitempty"` }
type listCoresOutput struct { Count int `json:"count"`; Items []BrowserCoreInfo `json:"items"` }

func registerProxyTools(srv *mcp.Server,p ProxyProvider){
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_proxy_list",Description:"列出代理池节点；不返回代理配置凭据。",Annotations:readOnly("列出代理节点")},func(_ context.Context,_ *mcp.CallToolRequest,in listProxiesInput)(*mcp.CallToolResult,listProxiesOutput,error){items,err:=p.ListProxies(); if err!=nil{return toolError(err),listProxiesOutput{},nil}; out:=make([]ProxyNode,0,len(items)); for _,item:=range items{if in.GroupName!=""&&item.GroupName!=in.GroupName{continue}; if in.AvailableOnly&&!item.LastTestOk{continue}; out=append(out,toProxyNode(item))}; return nil,listProxiesOutput{Count:len(out),Items:out},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_proxy_test_speed",Description:"对指定代理测速并保存结果。",Annotations:mutating("代理测速")},func(_ context.Context,_ *mcp.CallToolRequest,in proxyIDInput)(*mcp.CallToolResult,testProxySpeedOutput,error){r,err:=p.TestProxySpeed(in.ProxyID); if err!=nil{return toolError(err),testProxySpeedOutput{},nil}; return nil,testProxySpeedOutput{ProxyID:r.ProxyID,Ok:r.Ok,LatencyMs:r.LatencyMs,Engine:r.Engine,Error:r.Error},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_proxy_check_health",Description:"检测代理出口 IP 归属与风险信息。",Annotations:mutating("代理 IP 健康检测")},func(_ context.Context,_ *mcp.CallToolRequest,in proxyIDInput)(*mcp.CallToolResult,checkProxyHealthOutput,error){r,err:=p.CheckProxyHealth(in.ProxyID); if err!=nil{return toolError(err),checkProxyHealthOutput{},nil}; return nil,checkProxyHealthOutput{ProxyID:r.ProxyID,Ok:r.Ok,IP:r.IP,Country:r.Country,Region:r.Region,City:r.City,AsOrganization:r.AsOrganization,FraudScore:r.FraudScore,IsResidential:r.IsResidential,Source:r.Source,Error:r.Error},nil})
	mcp.AddTool(srv,&mcp.Tool{Name:"ant_core_list",Description:"列出已登记的浏览器内核。",Annotations:readOnly("列出浏览器内核")},func(_ context.Context,_ *mcp.CallToolRequest,_ struct{})(*mcp.CallToolResult,listCoresOutput,error){items,err:=p.ListCores(); if err!=nil{return toolError(err),listCoresOutput{},nil}; out:=make([]BrowserCoreInfo,0,len(items)); for _,item:=range items{out=append(out,BrowserCoreInfo{CoreID:item.CoreId,CoreName:item.CoreName,IsDefault:item.IsDefault})}; return nil,listCoresOutput{Count:len(out),Items:out},nil})
}
