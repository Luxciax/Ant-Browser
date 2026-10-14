package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
)

func (a *App) recordBridgeFailure(engine, key string, err error) {
	a.bridgeMu.Lock()
	ids := []string{}
	for id, ref := range a.profileBridgeRefs {
		if ref.Engine == engine && ref.Key == key {
			ids = append(ids, id)
		}
	}
	a.bridgeMu.Unlock()
	runs := map[string]string{}
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for _, id := range ids {
			if p := a.browserMgr.Profiles[id]; p != nil && p.Running {
				runs[id] = p.LastStartAt
				p.RuntimeWarning = fmt.Sprintf("代理连接中断（%s）：%v；出口配置保留，未降级直连", engine, err)
			}
		}
		a.browserMgr.Mutex.Unlock()
	}
	logger.New("ProxyRuntime").Error("代理桥接失效", logger.F("engine", engine), logger.F("bridge_key", key), logger.F("profile_ids", ids), logger.F("error", err))
	for id, run := range runs {
		a.recordDiagnosticIncident(id, run, "proxy.bridge_failed", map[string]interface{}{"engine": engine, "bridge_key": key, "error": errorMessage(err)})
	}
	a.emitRuntimeEvent("proxy:bridge:died", map[string]interface{}{"engine": engine, "key": bridgeEventKey(key), "profileIds": ids, "error": errorMessage(err)})
}

// Called by proxy watchers outside their manager locks, before recovery replaces the process.
func (a *App) recordBridgeEvidence(engine, key, event string, fields map[string]interface{}) {
	a.bridgeMu.Lock()
	ids := []string{}
	for id, ref := range a.profileBridgeRefs {
		if ref.Engine == engine && ref.Key == key {
			ids = append(ids, id)
		}
	}
	a.bridgeMu.Unlock()
	runs := map[string]string{}
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for _, id := range ids {
			if p := a.browserMgr.Profiles[id]; p != nil && p.Running {
				runs[id] = p.LastStartAt
			}
		}
		a.browserMgr.Mutex.Unlock()
	}
	for id, run := range runs {
		safe := map[string]interface{}{"engine": engine, "bridge_key": key}
		args := []logger.Field{logger.F("event", event), logger.F("critical", true), logger.F("profile_id", id), logger.F("run_id", run), logger.F("engine", engine), logger.F("bridge_key", key)}
		for k, v := range fields {
			safe[k] = v
			args = append(args, logger.F(k, v))
		}
		if event == "proxy.bridge_exited" {
			logger.New("ProxyRuntime").Error("代理进程退出，已保留恢复前证据", args...)
			a.recordDiagnosticIncident(id, run, event, safe)
		} else {
			logger.New("ProxyRuntime").Info("代理连接已恢复", args...)
		}
	}
}
