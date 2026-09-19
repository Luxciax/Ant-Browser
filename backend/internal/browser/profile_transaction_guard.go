package browser

import "fmt"

// CheckProfileFileTransactions blocks new writers after a failed file recovery.
// Stopping existing browsers remains available; restarting the app retries the
// recovery before managers or API servers are loaded.
func (m *Manager) CheckProfileFileTransactions() error {
	if dao, ok := m.ProfileDAO.(*SQLiteProfileDAO); ok {
		var pending int
		if err := dao.db.QueryRow(`SELECT count(*) FROM profile_file_transactions`).Scan(&pending); err != nil {
			return err
		}
		if pending != 0 {
			return fmt.Errorf("存在未完成的实例文件事务，请先重启应用完成恢复")
		}
	}
	return nil
}
