package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	portableLoginEnvelopeVersion = 1
	portableLoginKDF             = "argon2id"
	portableLoginMemoryKiB       = 64 * 1024
	portableLoginIterations      = 3
	portableLoginParallelism     = 4
	portableLoginKeySize         = 32
	portableLoginSaltSize        = 16
	localStateFileName           = "Local State"
)

var (
	errPortableLoginKeyMissing   = errors.New("browser os_crypt encrypted_key is missing")
	errPortableLoginUnsupported  = errors.New("portable login-state migration is unsupported on this platform")
	errPortableLoginBadPassword  = errors.New("portable login-state password is invalid")
	errPortableLoginInvalidState = errors.New("browser Local State is invalid")
)

type profilePortableLoginEnvelope struct {
	Version     int    `json:"version"`
	KDF         string `json:"kdf"`
	MemoryKiB   uint32 `json:"memoryKiB"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
	Salt        string `json:"salt"`
	Nonce       string `json:"nonce"`
	Ciphertext  string `json:"ciphertext"`
}

func createProfilePortableLoginEnvelope(userDataDir string, password string) (*profilePortableLoginEnvelope, error) {
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return nil, fmt.Errorf("portable login-state password must contain at least 8 characters")
	}

	key, err := readAndUnprotectProfileOSCryptKey(userDataDir)
	if err != nil {
		return nil, err
	}
	defer zeroSensitiveBytes(key)

	return sealPortableLoginKey(key, password)
}

func restoreProfilePortableLoginEnvelope(userDataDir string, password string, envelope profilePortableLoginEnvelope) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return errPortableLoginBadPassword
	}

	key, err := openPortableLoginKey(envelope, password)
	if err != nil {
		return err
	}
	defer zeroSensitiveBytes(key)

	protected, err := protectProfileOSCryptKey(key)
	if err != nil {
		return err
	}
	defer zeroSensitiveBytes(protected)

	encoded := make([]byte, 0, len("DPAPI")+len(protected))
	encoded = append(encoded, []byte("DPAPI")...)
	encoded = append(encoded, protected...)
	defer zeroSensitiveBytes(encoded)

	return rewriteProfileLocalStateEncryptedKey(userDataDir, base64.StdEncoding.EncodeToString(encoded))
}

func sealPortableLoginKey(key []byte, password string) (*profilePortableLoginEnvelope, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("portable login-state key is empty")
	}

	salt := make([]byte, portableLoginSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate portable-login salt: %w", err)
	}

	derived := argon2.IDKey([]byte(password), salt, portableLoginIterations, portableLoginMemoryKiB, portableLoginParallelism, portableLoginKeySize)
	defer zeroSensitiveBytes(derived)

	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, fmt.Errorf("create portable-login cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create portable-login gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate portable-login nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, key, nil)

	return &profilePortableLoginEnvelope{
		Version:     portableLoginEnvelopeVersion,
		KDF:         portableLoginKDF,
		MemoryKiB:   portableLoginMemoryKiB,
		Iterations:  portableLoginIterations,
		Parallelism: portableLoginParallelism,
		Salt:        base64.StdEncoding.EncodeToString(salt),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func openPortableLoginKey(envelope profilePortableLoginEnvelope, password string) ([]byte, error) {
	if envelope.Version != portableLoginEnvelopeVersion || envelope.KDF != portableLoginKDF {
		return nil, fmt.Errorf("unsupported portable login-state envelope")
	}
	if envelope.MemoryKiB < 8*1024 || envelope.MemoryKiB > 256*1024 || envelope.Iterations < 1 || envelope.Iterations > 10 || envelope.Parallelism < 1 || envelope.Parallelism > 16 {
		return nil, fmt.Errorf("portable login-state KDF parameters are outside supported limits")
	}

	salt, err := base64.StdEncoding.DecodeString(envelope.Salt)
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return nil, fmt.Errorf("portable login-state salt is invalid")
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return nil, fmt.Errorf("portable login-state nonce is invalid")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("portable login-state ciphertext is invalid")
	}

	derived := argon2.IDKey([]byte(password), salt, envelope.Iterations, envelope.MemoryKiB, envelope.Parallelism, portableLoginKeySize)
	defer zeroSensitiveBytes(derived)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, fmt.Errorf("create portable-login cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create portable-login gcm: %w", err)
	}
	if len(nonce) != gcm.NonceSize() || len(ciphertext) < gcm.Overhead() {
		return nil, fmt.Errorf("portable login-state envelope is malformed")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errPortableLoginBadPassword
	}
	return plaintext, nil
}

func readAndUnprotectProfileOSCryptKey(userDataDir string) ([]byte, error) {
	localStatePath := filepath.Join(userDataDir, localStateFileName)
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("read browser Local State: %w", err)
	}
	var state struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("%w: %v", errPortableLoginInvalidState, err)
	}
	encoded := strings.TrimSpace(state.OSCrypt.EncryptedKey)
	if encoded == "" {
		return nil, errPortableLoginKeyMissing
	}
	protected, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("browser os_crypt encrypted_key is not valid base64")
	}
	if len(protected) <= len("DPAPI") || string(protected[:len("DPAPI")]) != "DPAPI" {
		return nil, fmt.Errorf("browser os_crypt encrypted_key is not a supported DPAPI key")
	}
	return unprotectProfileOSCryptKey(protected[len("DPAPI"):])
}

func rewriteProfileLocalStateEncryptedKey(userDataDir string, encryptedKey string) error {
	path := filepath.Join(userDataDir, localStateFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read browser Local State: %w", err)
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("%w: %v", errPortableLoginInvalidState, err)
	}
	osCrypt, ok := state["os_crypt"].(map[string]any)
	if !ok {
		osCrypt = map[string]any{}
		state["os_crypt"] = osCrypt
	}
	osCrypt["encrypted_key"] = encryptedKey

	updated, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode browser Local State: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat browser Local State: %w", err)
	}
	if err := fsutil.AtomicWriteFile(path, updated, info.Mode().Perm()); err != nil {
		return fmt.Errorf("replace browser Local State: %w", err)
	}
	return nil
}

func zeroSensitiveBytes(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

func profilePortableLoginEntryName(profileID string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(profileID))))
	return fmt.Sprintf("portable-login/%x.json", sum[:])
}
