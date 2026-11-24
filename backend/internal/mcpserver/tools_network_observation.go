package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type networkObserveProvider interface {
	NetworkObserve(profileID,url string,timeoutMs int)(map[string]interface{},error)
	NetworkObservationGet(id string)(map[string]interface{},error)
}
type networkObserveInput struct{ ProfileID string `json:"profileId,omitempty"`; URL string `json:"url"`; TimeoutMs int `json:"timeoutMs,omitempty"` }

func registerNetworkObservationTools(srv *mcp.Server,p Provider){
	provider,ok:=any(p).(networkObserveProvider);if !ok{return}
	if providerAllowsScope(p,"page-read"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_network_observe",Description:"执行一次 host-source 网络观测，记录 DNS/TCP/TLS/ALPN/HTTP 协议；不会伪造 JA3/JA4/QUIC 证据。",Annotations:readOnly("执行网络观测")},func(_ context.Context,_ *mcp.CallToolRequest,in networkObserveInput)(*mcp.CallToolResult,phase2Output,error){
			v,e:=provider.NetworkObserve(in.ProfileID,in.URL,in.TimeoutMs);if e!=nil{return toolError(e),phase2Output{Data:v},nil};return nil,phase2Output{Data:v},nil
		})
	}
	if providerAllowsScope(p,"environment-read"){
		mcp.AddTool(srv,&mcp.Tool{Name:"ant_network_observation_get",Description:"读取已持久化的网络观测证据，不触发新网络请求。",Annotations:readOnly("读取网络观测")},func(_ context.Context,_ *mcp.CallToolRequest,in struct{ID string `json:"id"`})(*mcp.CallToolResult,phase2Output,error){
			v,e:=provider.NetworkObservationGet(in.ID);if e!=nil{return toolError(e),phase2Output{},nil};return nil,phase2Output{Data:v},nil
		})
	}
}
