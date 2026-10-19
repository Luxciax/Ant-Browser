package config

// FingerprintBackend identifies the native switch contract, independently of Chromium version.
type FingerprintBackend string

const (
	FingerprintBackendGenericChromium     FingerprintBackend = "generic-chromium"
	FingerprintBackendFingerprintChromium FingerprintBackend = "fingerprint-chromium"
	FingerprintBackendClearcote           FingerprintBackend = "clearcote"
)

func (b FingerprintBackend) Valid() bool {
	return b == FingerprintBackendGenericChromium || b == FingerprintBackendFingerprintChromium || b == FingerprintBackendClearcote
}

// Existing records without metadata keep their original launch contract.
func (b FingerprintBackend) LegacyDefault() FingerprintBackend {
	if b.Valid() {
		return b
	}
	return FingerprintBackendFingerprintChromium
}
