package browser

import (
	"ant-chrome/backend/internal/config"
	"testing"
)

func TestCoreBackendPersistence(t *testing.T) {
	dao, _ := newCoreDAOTestDB(t)
	for _, backend := range []config.FingerprintBackend{"", config.FingerprintBackendClearcote, config.FingerprintBackendGenericChromium} {
		if err := dao.Upsert(Core{CoreId: "core", CoreName: "Core", CorePath: "chrome", FingerprintBackend: backend}); err != nil {
			t.Fatal(err)
		}
		cores, err := dao.List()
		if err != nil || len(cores) != 1 || cores[0].FingerprintBackend != backend {
			t.Fatalf("backend not persisted: %+v, %v", cores, err)
		}
	}
	if got := config.FingerprintBackend("").LegacyDefault(); got != config.FingerprintBackendFingerprintChromium {
		t.Fatalf("legacy contract changed: %s", got)
	}
}
