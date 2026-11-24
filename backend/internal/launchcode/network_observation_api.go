package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type NetworkObservationRequest struct {
	ProfileID string `json:"profileId,omitempty"`
	URL       string `json:"url"`
	TimeoutMs int    `json:"timeoutMs,omitempty"`
}

type NetworkObservationProvider interface {
	BrowserNetworkObserve(req interface{}) (interface{}, error)
	BrowserNetworkObservationGet(id string) (interface{}, error)
}

func (s *LaunchServer) handleNetworkObserve(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	var req NetworkObservationRequest
	dec:=json.NewDecoder(io.LimitReader(r.Body,1<<20));dec.DisallowUnknownFields()
	if err:=dec.Decode(&req);err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":"invalid request body"});return}
	provider,ok:=s.starter.(interface{ BrowserControlNetworkObserve(profileID,url string,timeoutMs int)(map[string]interface{},error) })
	if !ok{writeJSON(w,503,map[string]any{"ok":false,"error":"network observation is unavailable"});return}
	value,err:=provider.BrowserControlNetworkObserve(req.ProfileID,req.URL,req.TimeoutMs)
	if err!=nil{writeJSON(w,502,map[string]any{"ok":false,"error":err.Error(),"observation":value});return}
	writeJSON(w,200,map[string]any{"ok":true,"observation":value})
}

func (s *LaunchServer) handleNetworkObservationGet(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	id:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/network/observations/"),"/")
	provider,ok:=s.starter.(interface{ BrowserControlNetworkObservationGet(id string)(map[string]interface{},error) })
	if !ok{writeJSON(w,503,map[string]any{"ok":false,"error":"network observation is unavailable"});return}
	value,err:=provider.BrowserControlNetworkObservationGet(id)
	if err!=nil{writeJSON(w,404,map[string]any{"ok":false,"error":err.Error()});return}
	writeJSON(w,200,map[string]any{"ok":true,"observation":value})
}
