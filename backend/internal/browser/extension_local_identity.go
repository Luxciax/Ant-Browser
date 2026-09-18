package browser

import (
	"ant-chrome/backend/internal/fsutil"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const localExtensionPrivateKeyBits = 2048

type localExtensionIdentity struct {
	ExtensionID string
	PublicKey   []byte
	KeyPath     string
	CreatedKey  bool
}

func (m *Manager) resolveLocalExtensionIdentity(sourceDir string) (localExtensionIdentity, string, error) {
	canonicalSource, err := canonicalLocalExtensionSourceDir(sourceDir)
	if err != nil {
		return localExtensionIdentity{}, "", err
	}

	if m != nil && m.ExtensionDAO != nil {
		extensions, listErr := m.ExtensionDAO.List()
		if listErr != nil {
			return localExtensionIdentity{}, "", fmt.Errorf("读取本地插件目录映射失败: %w", listErr)
		}
		var fallback *localExtensionIdentity
		for _, extension := range extensions {
			if !sameLocalExtensionSource(extension.SourceURL, canonicalSource) {
				continue
			}
			identity, loadErr := m.loadLocalExtensionIdentity(extension.ExtensionID)
			if loadErr != nil {
				continue
			}
			if strings.EqualFold(identity.ExtensionID, extension.ExtensionID) {
				return identity, canonicalSource, nil
			}
			copyIdentity := identity
			fallback = &copyIdentity
		}
		if fallback != nil {
			if err := m.ensureLocalExtensionKeyAtID(*fallback); err != nil {
				return localExtensionIdentity{}, "", err
			}
			fallback.KeyPath = m.localExtensionKeyPath(fallback.ExtensionID)
			return *fallback, canonicalSource, nil
		}
	}

	identity, err := m.createLocalExtensionIdentity()
	if err != nil {
		return localExtensionIdentity{}, "", err
	}
	return identity, canonicalSource, nil
}

func (m *Manager) createLocalExtensionIdentity() (localExtensionIdentity, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, localExtensionPrivateKeyBits)
	if err != nil {
		return localExtensionIdentity{}, fmt.Errorf("生成本地插件私钥失败: %w", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return localExtensionIdentity{}, fmt.Errorf("编码本地插件公钥失败: %w", err)
	}
	extensionID := extensionIDFromPublicKeyBytes(publicDER)
	if extensionID == "" {
		return localExtensionIdentity{}, fmt.Errorf("生成本地插件 ID 失败")
	}
	keyPath := m.localExtensionKeyPath(extensionID)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return localExtensionIdentity{}, fmt.Errorf("创建本地插件密钥目录失败: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return localExtensionIdentity{}, fmt.Errorf("编码本地插件私钥失败: %w", err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	if len(encoded) == 0 {
		return localExtensionIdentity{}, fmt.Errorf("编码本地插件私钥失败")
	}
	if err := fsutil.AtomicWriteFile(keyPath, encoded, 0o600); err != nil {
		return localExtensionIdentity{}, fmt.Errorf("保存本地插件私钥失败: %w", err)
	}
	return localExtensionIdentity{
		ExtensionID: extensionID,
		PublicKey:   publicDER,
		KeyPath:     keyPath,
		CreatedKey:  true,
	}, nil
}

func (m *Manager) loadLocalExtensionIdentity(extensionID string) (localExtensionIdentity, error) {
	keyPath := m.localExtensionKeyPath(extensionID)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return localExtensionIdentity{}, err
	}
	privateKey, publicDER, err := parseLocalExtensionPrivateKey(data)
	if err != nil {
		return localExtensionIdentity{}, err
	}
	_ = privateKey
	derivedID := extensionIDFromPublicKeyBytes(publicDER)
	if derivedID == "" {
		return localExtensionIdentity{}, fmt.Errorf("无法从本地插件私钥派生 ID")
	}
	return localExtensionIdentity{ExtensionID: derivedID, PublicKey: publicDER, KeyPath: keyPath}, nil
}

func (m *Manager) ensureLocalExtensionKeyAtID(identity localExtensionIdentity) error {
	if strings.TrimSpace(identity.KeyPath) == "" || strings.TrimSpace(identity.ExtensionID) == "" {
		return fmt.Errorf("本地插件密钥信息不完整")
	}
	targetPath := m.localExtensionKeyPath(identity.ExtensionID)
	if sameExtensionPath(identity.KeyPath, targetPath) {
		return nil
	}
	data, err := os.ReadFile(identity.KeyPath)
	if err != nil {
		return fmt.Errorf("读取本地插件私钥失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("创建本地插件密钥目录失败: %w", err)
	}
	if err := fsutil.AtomicWriteFile(targetPath, data, 0o600); err != nil {
		return fmt.Errorf("迁移本地插件私钥失败: %w", err)
	}
	return nil
}

func (m *Manager) localExtensionKeyPath(extensionID string) string {
	return m.ResolveRelativePath(filepath.Join("data", extensionsRootDir, "packages", strings.TrimSpace(extensionID)+".pem"))
}

func parseLocalExtensionPrivateKey(data []byte) (*rsa.PrivateKey, []byte, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, fmt.Errorf("本地插件私钥不是有效 PEM")
	}
	var privateKey *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := parsed.(*rsa.PrivateKey); ok {
			privateKey = rsaKey
		}
	}
	if privateKey == nil {
		if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			privateKey = rsaKey
		}
	}
	if privateKey == nil {
		return nil, nil, fmt.Errorf("本地插件 PEM 不包含 RSA 私钥")
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("编码本地插件公钥失败: %w", err)
	}
	return privateKey, publicDER, nil
}

func extensionIDFromPublicKeyBytes(publicKey []byte) string {
	if len(publicKey) == 0 {
		return ""
	}
	sum := sha256.Sum256(publicKey)
	hexValue := hex.EncodeToString(sum[:16])
	var builder strings.Builder
	builder.Grow(len(hexValue))
	for _, char := range hexValue {
		if char >= '0' && char <= '9' {
			builder.WriteByte(byte('a' + char - '0'))
			continue
		}
		builder.WriteByte(byte('k' + char - 'a'))
	}
	return builder.String()
}

func manifestWithLocalExtensionKey(manifestData []byte, publicKey []byte) ([]byte, error) {
	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析本地插件 manifest.json 失败: %w", err)
	}
	manifest["key"] = base64.StdEncoding.EncodeToString(publicKey)
	updated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("写入本地插件稳定公钥失败: %w", err)
	}
	return updated, nil
}

func canonicalLocalExtensionSourceDir(sourceDir string) (string, error) {
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return "", fmt.Errorf("插件目录不能为空")
	}
	absPath, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", fmt.Errorf("解析插件目录失败: %w", err)
	}
	absPath = filepath.Clean(absPath)
	if resolved, evalErr := filepath.EvalSymlinks(absPath); evalErr == nil {
		absPath = filepath.Clean(resolved)
	}
	return absPath, nil
}

func sameLocalExtensionSource(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" || strings.Contains(left, "://") || strings.Contains(right, "://") {
		return false
	}
	leftPath, leftErr := canonicalLocalExtensionSourceDir(left)
	rightPath, rightErr := canonicalLocalExtensionSourceDir(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return sameExtensionPath(leftPath, rightPath)
}

func sameExtensionPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	return left != "" && right != "" && strings.EqualFold(left, right)
}
