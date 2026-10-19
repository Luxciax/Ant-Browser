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

func TestCoreSaveLegacyCompatibilityAndExplicitOverride(t *testing.T) {
	root := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []Core{{CoreId: "existing", CoreName: "Existing kernel", CorePath: "legacy", IsDefault: true}}
	manager := NewManager(cfg, root)
	if err := manager.SaveCore(CoreInput{CoreId: "existing", CoreName: "Renamed", CorePath: "legacy"}); err != nil {
		t.Fatal(err)
	}
	existing, _ := manager.GetCore("existing")
	if existing.FingerprintBackend != config.FingerprintBackendFingerprintChromium {
		t.Fatal("old records changed backend")
	}
	if err := manager.SaveCore(CoreInput{CoreId: "unknown", CoreName: "clearcote is only a name", CorePath: "missing"}); err != nil {
		t.Fatal(err)
	}
	unknown, _ := manager.GetCore("unknown")
	if unknown.FingerprintBackend != config.FingerprintBackendGenericChromium {
		t.Fatalf("name guessed a backend: %s", unknown.FingerprintBackend)
	}
	if err := manager.SaveCore(CoreInput{CoreId: "unknown", CoreName: "Custom kernel", CorePath: "missing", FingerprintBackend: config.FingerprintBackendClearcote}); err != nil {
		t.Fatal(err)
	}
	custom, _ := manager.GetCore("unknown")
	if custom.FingerprintBackend != config.FingerprintBackendClearcote {
		t.Fatal("explicit backend was ignored")
	}
}
