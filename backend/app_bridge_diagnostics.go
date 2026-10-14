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
	if a.browserMgr != nil {
		a.browserMgr.Mutex.Lock()
		for _, id := range ids {
			if p := a.browserMgr.Profiles[id]; p != nil && p.Running {
				p.RuntimeWarning = fmt.Sprintf("代理连接中断（%s）：%v；出口配置保留，未降级直连", engine, err)
			}
		}
		a.browserMgr.Mutex.Unlock()
	}
	logger.New("ProxyRuntime").Error("代理桥接失效", logger.F("engine", engine), logger.F("bridge_key", key), logger.F("profile_ids", ids), logger.F("error", err))
	a.emitRuntimeEvent("proxy:bridge:died", map[string]interface{}{"engine": engine, "key": bridgeEventKey(key), "profileIds": ids, "error": errorMessage(err)})
}
