package launchcode

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/browser"
)

type BrowserPhase4Provider interface {
	BrowserControlEnvironmentView(profileID string) (map[string]interface{}, error)
	BrowserControlDiagnostics() (map[string]interface{}, error)
	BrowserControlBatchOperation(operationID string) (map[string]interface{}, error)
	BrowserControlBatchCreate(inputs []browser.ProfileInput, clientKeys []string, concurrency int) (map[string]interface{}, error)
	BrowserControlBatchStart(profileIDs []string, concurrency int) (map[string]interface{}, error)
	BrowserControlBatchStop(profileIDs []string, concurrency int) (map[string]interface{}, error)
}

type batchCreateItem struct {
	ClientKey string               `json:"clientKey,omitempty"`
	Config    browser.ProfileInput `json:"config"`
}
type batchCreateRequest struct {
	Items []batchCreateItem `json:"items"`
	Concurrency int         `json:"concurrency,omitempty"`
}

type batchProfilesRequest struct {
	ProfileIDs  []string `json:"profileIds"`
	Concurrency int      `json:"concurrency,omitempty"`
}

func (s *LaunchServer) phase4Provider() (BrowserPhase4Provider, error) {
	if s == nil || s.starter == nil {
		return nil, newServiceError(http.StatusServiceUnavailable, "phase4 control is unavailable")
	}
	provider, ok := s.starter.(BrowserPhase4Provider)
	if !ok {
		return nil, newServiceError(http.StatusServiceUnavailable, "phase4 control is unavailable")
	}
	return provider, nil
}

func (s *LaunchServer) EnvironmentView(profileID string) (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlEnvironmentView(profileID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) Diagnostics() (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlDiagnostics()
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) BatchOperation(operationID string) (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlBatchOperation(operationID)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) BatchCreate(inputs []browser.ProfileInput, clientKeys []string, concurrency int) (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlBatchCreate(inputs, clientKeys, concurrency)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) BatchStart(profileIDs []string, concurrency int) (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlBatchStart(profileIDs, concurrency)
	return value, phase2ServiceError(err)
}

func (s *LaunchServer) BatchStop(profileIDs []string, concurrency int) (map[string]interface{}, error) {
	p, err := s.phase4Provider(); if err != nil { return nil, err }
	value, err := p.BrowserControlBatchStop(profileIDs, concurrency)
	return value, phase2ServiceError(err)
}

func decodeBatchCreateRequest(r *http.Request) (batchCreateRequest, error) {
	var req batchCreateRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil { return req, err }
	return req, nil
}

func decodeBatchProfilesRequest(r *http.Request) (batchProfilesRequest, error) {
	var req batchProfilesRequest
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil { return req, err }
	return req, nil
}

func (s *LaunchServer) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"ok":false,"error":"method not allowed"}); return
	}
	value, err := s.Diagnostics()
	if err != nil { writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok":false,"error":err.Error()}); return }
	writeJSON(w, http.StatusOK, value)
}

func (s *LaunchServer) handleBatch(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/batch/"), "/")
	if path == "" { writeJSON(w,http.StatusNotFound,map[string]interface{}{"ok":false,"error":"batch action not found"}); return }
	switch {
	case path == "create":
		if r.Method != http.MethodPost { writeJSON(w,http.StatusMethodNotAllowed,map[string]interface{}{"ok":false,"error":"method not allowed"}); return }
		req, err := decodeBatchCreateRequest(r)
		if err != nil { writeJSON(w,http.StatusBadRequest,map[string]interface{}{"ok":false,"error":"invalid request body"}); return }
		inputs := make([]browser.ProfileInput,0,len(req.Items)); keys:=make([]string,0,len(req.Items))
		for _, item := range req.Items { inputs=append(inputs,item.Config); keys=append(keys,item.ClientKey) }
		value, err := s.BatchCreate(inputs,keys,req.Concurrency)
		if err != nil { writeJSON(w,http.StatusBadRequest,map[string]interface{}{"ok":false,"error":err.Error()}); return }
		value["ok"]=true; writeJSON(w,http.StatusOK,value)
	case path == "start" || path == "stop":
		if r.Method != http.MethodPost { writeJSON(w,http.StatusMethodNotAllowed,map[string]interface{}{"ok":false,"error":"method not allowed"}); return }
		req, err := decodeBatchProfilesRequest(r)
		if err != nil { writeJSON(w,http.StatusBadRequest,map[string]interface{}{"ok":false,"error":"invalid request body"}); return }
		var value map[string]interface{}
		if path == "start" { value, err = s.BatchStart(req.ProfileIDs, req.Concurrency) } else { value, err = s.BatchStop(req.ProfileIDs, req.Concurrency) }
		if err != nil { writeJSON(w,http.StatusBadRequest,map[string]interface{}{"ok":false,"error":err.Error()}); return }
		value["ok"] = true
		writeJSON(w,http.StatusOK,value)
	default:
		if r.Method != http.MethodGet { writeJSON(w,http.StatusMethodNotAllowed,map[string]interface{}{"ok":false,"error":"method not allowed"}); return }
		value, err := s.BatchOperation(path)
		if err != nil { writeJSON(w,http.StatusNotFound,map[string]interface{}{"ok":false,"error":err.Error()}); return }
		value["ok"] = true
		writeJSON(w,http.StatusOK,value)
	}
}
