package backend

import (
	"ant-chrome/backend/internal/profiletxn"
	"strings"
)

func (a *App) profileTransactionRoots() []string {
	root := strings.TrimSpace(a.config.Browser.UserDataRoot)
	if root == "" {
		root = "data"
	}
	return []string{a.resolveAppPath("data"), a.resolveAppPath(root)}
}

func (a *App) recoverProfileFileTransactions() error {
	return profiletxn.Recover(a.db.GetConn(), a.profileTransactionRoots())
}
