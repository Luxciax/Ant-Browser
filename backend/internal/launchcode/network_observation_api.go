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

type networkObservationControlProvider interface {
	BrowserControlNetworkObserve(profileID,url string,timeoutMs int)(map[string]interface{},error)
	BrowserControlNetworkObservationGet(id string)(map[string]interface{},error)
}

func (s *LaunchServer) NetworkObserve(profileID,url string,timeoutMs int)(map[string]interface{},error){
	provider,ok:=s.starter.(networkObservationControlProvider)
	if !ok{return nil,newServiceError(http.StatusServiceUnavailable,"network observation is unavailable")}
	return provider.BrowserControlNetworkObserve(profileID,url,timeoutMs)
}
func (s *LaunchServer) NetworkObservationGet(id string)(map[string]interface{},error){
	provider,ok:=s.starter.(networkObservationControlProvider)
	if !ok{return nil,newServiceError(http.StatusServiceUnavailable,"network observation is unavailable")}
	return provider.BrowserControlNetworkObservationGet(id)
}

func (s *LaunchServer) handleNetworkObserve(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	var req NetworkObservationRequest
	dec:=json.NewDecoder(io.LimitReader(r.Body,1<<20));dec.DisallowUnknownFields()
	if err:=dec.Decode(&req);err!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":"invalid request body"});return}
	value,err:=s.NetworkObserve(req.ProfileID,req.URL,req.TimeoutMs)
	if err!=nil{writeJSON(w,502,map[string]any{"ok":false,"error":err.Error(),"observation":value});return}
	writeJSON(w,200,map[string]any{"ok":true,"observation":value})
}

func (s *LaunchServer) handleNetworkObservationGet(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	id:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/network/observations/"),"/")
	value,err:=s.NetworkObservationGet(id)
	if err!=nil{writeJSON(w,404,map[string]any{"ok":false,"error":err.Error()});return}
	writeJSON(w,200,map[string]any{"ok":true,"observation":value})
}
