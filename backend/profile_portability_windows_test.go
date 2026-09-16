//go:build windows

package backend

import (
	"bytes"
	"testing"
)

func TestProfileDPAPIRoundTrip(t *testing.T) {
	original := []byte("ant-browser-portable-login-test-key")
	protected, err := protectProfileOSCryptKey(original)
	if err != nil {
		t.Fatalf("protectProfileOSCryptKey: %v", err)
	}
	defer zeroSensitiveBytes(protected)
	plain, err := unprotectProfileOSCryptKey(protected)
	if err != nil {
		t.Fatalf("unprotectProfileOSCryptKey: %v", err)
	}
	defer zeroSensitiveBytes(plain)
	if !bytes.Equal(plain, original) {
		t.Fatal("DPAPI round trip changed plaintext")
	}
}
