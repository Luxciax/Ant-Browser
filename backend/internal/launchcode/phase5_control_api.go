package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type BrowserPhase5ControlProvider interface {
	BrowserControlTrashList() (map[string]interface{}, error)
	BrowserControlTrashRestore(profileID string) (map[string]interface{}, error)
	BrowserControlTrashPermanentlyDelete(profileID string) error
	BrowserControlTrashCleanup() error
	BrowserControlSyncStatus() (map[string]interface{}, error)
	BrowserSystemIntegrationStatusGet() interface{}
	BrowserSystemIntegrationSetProtocol(enabled bool) (interface{}, error)
}
type systemProtocolRequest struct { Enabled bool `json:"enabled"` }

func (s *LaunchServer) phase5ControlProvider() (BrowserPhase5ControlProvider,error) {
	p,ok:=s.starter.(BrowserPhase5ControlProvider)
	if !ok{return nil,newServiceError(http.StatusServiceUnavailable,"phase5 control is unavailable")}
	return p,nil
}
func (s *LaunchServer) TrashList()(map[string]interface{},error){p,e:=s.phase5ControlProvider();if e!=nil{return nil,e};return p.BrowserControlTrashList()}
func (s *LaunchServer) TrashRestore(id string)(map[string]interface{},error){p,e:=s.phase5ControlProvider();if e!=nil{return nil,e};return p.BrowserControlTrashRestore(id)}
func (s *LaunchServer) TrashPermanentlyDelete(id string)error{p,e:=s.phase5ControlProvider();if e!=nil{return e};return p.BrowserControlTrashPermanentlyDelete(id)}
func (s *LaunchServer) TrashCleanup()error{p,e:=s.phase5ControlProvider();if e!=nil{return e};return p.BrowserControlTrashCleanup()}
func (s *LaunchServer) SyncStatus()(map[string]interface{},error){p,e:=s.phase5ControlProvider();if e!=nil{return nil,e};return p.BrowserControlSyncStatus()}

func (s *LaunchServer) handleSyncStatus(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodGet{writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"});return}
	v,e:=s.SyncStatus();if e!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":e.Error()});return};v["ok"]=true;writeJSON(w,200,v)
}
func (s *LaunchServer) handleTrash(w http.ResponseWriter,r *http.Request){
	if r.URL.Path=="/api/trash"||r.URL.Path=="/api/trash/"{
		switch r.Method{
		case http.MethodGet:
			v,e:=s.TrashList();if e!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":e.Error()});return};v["ok"]=true;writeJSON(w,200,v)
		case http.MethodPost:
			if r.URL.Query().Get("action")!="cleanup"{writeJSON(w,400,map[string]any{"ok":false,"error":"unsupported trash action"});return}
			if e:=s.TrashCleanup();e!=nil{writeJSON(w,500,map[string]any{"ok":false,"error":e.Error()});return};writeJSON(w,200,map[string]any{"ok":true})
		default:writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"})
		}
		return
	}
	path:=strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/trash/"),"/")
	parts:=strings.Split(path,"/")
	if len(parts)==2&&parts[1]=="restore"&&r.Method==http.MethodPost{
		v,e:=s.TrashRestore(parts[0]);if e!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":e.Error()});return};v["ok"]=true;writeJSON(w,200,v);return
	}
	if len(parts)==1&&r.Method==http.MethodDelete{
		if e:=s.TrashPermanentlyDelete(parts[0]);e!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":e.Error()});return};writeJSON(w,200,map[string]any{"ok":true});return
	}
	writeJSON(w,404,map[string]any{"ok":false,"error":"trash item not found"})
}
func (s *LaunchServer) handleSystemIntegration(w http.ResponseWriter,r *http.Request){
	p,e:=s.phase5ControlProvider();if e!=nil{writeJSON(w,503,map[string]any{"ok":false,"error":e.Error()});return}
	switch r.Method{
	case http.MethodGet:
		writeJSON(w,200,map[string]any{"ok":true,"status":p.BrowserSystemIntegrationStatusGet()})
	case http.MethodPost:
		var req systemProtocolRequest
		dec:=json.NewDecoder(io.LimitReader(r.Body,1<<20));dec.DisallowUnknownFields()
		if e:=dec.Decode(&req);e!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":"invalid request body"});return}
		v,e:=p.BrowserSystemIntegrationSetProtocol(req.Enabled);if e!=nil{writeJSON(w,400,map[string]any{"ok":false,"error":e.Error(),"status":v});return}
		writeJSON(w,200,map[string]any{"ok":true,"status":v})
	default:writeJSON(w,405,map[string]any{"ok":false,"error":"method not allowed"})
	}
}
