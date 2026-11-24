//go:build windows

package secretstore

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const dpapiProvider = "dpapi-user-v1"
const cryptProtectUIForbidden = 0x1

var (
	crypt32SecretStore = windows.NewLazySystemDLL("crypt32.dll")
	kernel32SecretStore = windows.NewLazySystemDLL("kernel32.dll")
	procSecretProtect = crypt32SecretStore.NewProc("CryptProtectData")
	procSecretUnprotect = crypt32SecretStore.NewProc("CryptUnprotectData")
	procSecretLocalFree = kernel32SecretStore.NewProc("LocalFree")
)

type secretDataBlob struct {
	Size uint32
	Data *byte
}

func protectValue(plaintext []byte) (string, string, error) {
	protected, err := callSecretDPAPI(procSecretProtect, plaintext)
	if err != nil {
		return "", "", err
	}
	defer zeroBytes(protected)
	return dpapiProvider, base64.StdEncoding.EncodeToString(protected), nil
}

func unprotectValue(provider, payload string) ([]byte, error) {
	if provider != dpapiProvider {
		return nil, fmt.Errorf("unsupported secret protection provider %q", provider)
	}
	protected, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid protected secret payload")
	}
	defer zeroBytes(protected)
	return callSecretDPAPI(procSecretUnprotect, protected)
}

func callSecretDPAPI(proc *windows.LazyProc, input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("secret value is empty")
	}
	in := secretDataBlob{Size: uint32(len(input)), Data: &input[0]}
	var out secretDataBlob
	r1, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0,
		cryptProtectUIForbidden, uintptr(unsafe.Pointer(&out)),
	)
	runtime.KeepAlive(input)
	if r1 == 0 {
		if callErr != nil && callErr != windows.ERROR_SUCCESS {
			return nil, fmt.Errorf("Windows DPAPI failed: %w", callErr)
		}
		return nil, fmt.Errorf("Windows DPAPI failed")
	}
	if out.Data == nil || out.Size == 0 {
		return nil, fmt.Errorf("Windows DPAPI returned empty output")
	}
	defer procSecretLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}
