package backend

import (
	"bytes"
	"testing"
)

func TestPortableLoginEnvelopeRoundTrip(t *testing.T) {
	original := []byte("0123456789abcdef0123456789abcdef")
	envelope, err := sealPortableLoginKey(original, "correct horse battery staple")
	if err != nil {
		t.Fatalf("sealPortableLoginKey: %v", err)
	}
	decoded, err := openPortableLoginKey(*envelope, "correct horse battery staple")
	if err != nil {
		t.Fatalf("openPortableLoginKey: %v", err)
	}
	defer zeroSensitiveBytes(decoded)
	if !bytes.Equal(decoded, original) {
		t.Fatalf("decoded key differs from original")
	}
}

func TestPortableLoginEnvelopeRejectsWrongPassword(t *testing.T) {
	envelope, err := sealPortableLoginKey([]byte("0123456789abcdef0123456789abcdef"), "correct-password")
	if err != nil {
		t.Fatalf("sealPortableLoginKey: %v", err)
	}
	if _, err := openPortableLoginKey(*envelope, "wrong-password"); err != errPortableLoginBadPassword {
		t.Fatalf("wrong password error = %v, want %v", err, errPortableLoginBadPassword)
	}
}
