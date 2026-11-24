//go:build !windows

package secretstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const envProvider = "aes-gcm-env-v1"
const envMasterKeyName = "ANT_BROWSER_SECRET_KEY"

func secretEnvKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(envMasterKeyName))
	if len(raw) < 16 {
		return nil, fmt.Errorf("%w: set %s to a high-entropy secret", ErrUnavailable, envMasterKeyName)
	}
	sum := sha256.Sum256([]byte(raw))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key, nil
}

func protectValue(plaintext []byte) (string, string, error) {
	key, err := secretEnvKey()
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, ciphertext...)
	return envProvider, base64.StdEncoding.EncodeToString(payload), nil
}

func unprotectValue(provider, payload string) ([]byte, error) {
	if provider != envProvider {
		return nil, fmt.Errorf("unsupported secret protection provider %q", provider)
	}
	key, err := secretEnvKey()
	if err != nil {
		return nil, err
	}
	defer zeroBytes(key)
	encoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid protected secret payload")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(encoded) < gcm.NonceSize()+gcm.Overhead() {
		return nil, fmt.Errorf("protected secret payload is malformed")
	}
	nonce := encoded[:gcm.NonceSize()]
	ciphertext := encoded[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secret decryption failed")
	}
	return plaintext, nil
}
