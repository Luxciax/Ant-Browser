//go:build windows

package backend

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const cryptProtectUIForbidden = 0x1

var (
	crypt32PortableLogin   = windows.NewLazySystemDLL("crypt32.dll")
	kernel32PortableLogin  = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtectData   = crypt32PortableLogin.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32PortableLogin.NewProc("CryptUnprotectData")
	procLocalFree          = kernel32PortableLogin.NewProc("LocalFree")
)

type portableLoginDataBlob struct {
	Size uint32
	Data *byte
}

func protectProfileOSCryptKey(plaintext []byte) ([]byte, error) {
	return callProfileDPAPI(procCryptProtectData, plaintext)
}

func unprotectProfileOSCryptKey(ciphertext []byte) ([]byte, error) {
	return callProfileDPAPI(procCryptUnprotectData, ciphertext)
}

func callProfileDPAPI(proc *windows.LazyProc, input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("DPAPI input is empty")
	}
	in := portableLoginDataBlob{Size: uint32(len(input)), Data: &input[0]}
	var out portableLoginDataBlob
	r1, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
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
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}
