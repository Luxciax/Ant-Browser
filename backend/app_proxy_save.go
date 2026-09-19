package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"ant-chrome/backend/internal/proxy"
)

func (a *App) SaveBrowserProxies(proxies []BrowserProxy) error {
	log := logger.New("Browser")
	normalized := proxy.NormalizeBrowserProxies(proxies, generateUUID)

	if a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.ReplaceAll(normalized); err != nil {
			log.Error("代理保存事务失败", logger.F("error", err))
			return err
		}
		log.Info("代理列表已保存到数据库", logger.F("count", len(normalized)))
	} else {
		if err := config.SaveProxies(a.resolveAppPath("proxies.yaml"), normalized); err != nil {
			log.Error("代理列表保存失败", logger.F("error", err))
			return err
		}
	}

	a.config.Browser.Proxies = normalized
	a.reconcileProfileProxyBindings()
	return nil
}
