package config

import "testing"

func TestDiagnosticLoggingMigratesLegacyDefaultsOnce(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ConfigVersion = 2
	cfg.Logging.FileEnabled = false
	cfg.Logging.Format = "text"
	cfg.Logging.Rotation.Enabled = false
	if !MigrateLegacyConfig(cfg) || !cfg.Logging.FileEnabled || cfg.Logging.Format != "json" || !cfg.Logging.Rotation.Enabled {
		t.Fatal("legacy default logs not migrated")
	}
	cfg.Logging.FileEnabled = false
	if MigrateLegacyConfig(cfg) || cfg.Logging.FileEnabled {
		t.Fatal("explicit opt-out was overridden")
	}
}

func TestDiagnosticMigrationPreservesCustomLogSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ConfigVersion = 2
	cfg.Logging.FileEnabled = false
	cfg.Logging.FilePath = "custom.log"
	cfg.Logging.Format = "text"
	MigrateLegacyConfig(cfg)
	if cfg.Logging.FileEnabled || cfg.Logging.FilePath != "custom.log" {
		t.Fatal("custom logging settings changed")
	}
}
