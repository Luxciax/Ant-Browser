package browser

import (
	"path/filepath"
	"testing"

	"ant-chrome/backend/internal/database"
)

func TestSQLiteProxyDAOReplaceAllRollsBackOnInsertFailure(t *testing.T) {
	db, err := database.NewDB(filepath.Join(t.TempDir(), "proxies.db"))
	if err != nil {
		t.Fatalf("NewDB returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}

	dao := NewSQLiteProxyDAO(db.GetConn())
	original := Proxy{ProxyId: "old-proxy", ProxyName: "Old", ProxyConfig: "http://127.0.0.1:8080"}
	if err := dao.Upsert(original); err != nil {
		t.Fatalf("seed proxy: %v", err)
	}
	if _, err := db.GetConn().Exec(`
		CREATE TRIGGER fail_bad_proxy
		BEFORE INSERT ON browser_proxies
		WHEN NEW.proxy_id = 'bad-proxy'
		BEGIN
			SELECT RAISE(ABORT, 'forced proxy insert failure');
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	err = dao.ReplaceAll([]Proxy{
		{ProxyId: "new-proxy", ProxyName: "New", ProxyConfig: "http://127.0.0.1:8081"},
		{ProxyId: "bad-proxy", ProxyName: "Bad", ProxyConfig: "http://127.0.0.1:8082"},
	})
	if err == nil {
		t.Fatal("ReplaceAll returned nil error, want transaction failure")
	}

	proxies, err := dao.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(proxies) != 1 || proxies[0].ProxyId != original.ProxyId {
		t.Fatalf("proxies after rollback = %#v, want original proxy only", proxies)
	}
}
