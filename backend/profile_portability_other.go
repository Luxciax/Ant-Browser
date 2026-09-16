//go:build !windows

package backend

func protectProfileOSCryptKey([]byte) ([]byte, error) {
	return nil, errPortableLoginUnsupported
}

func unprotectProfileOSCryptKey([]byte) ([]byte, error) {
	return nil, errPortableLoginUnsupported
}
