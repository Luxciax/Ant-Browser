package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

type BrowserKernelCapabilityManifest struct {
	SchemaVersion  int                              `json:"schemaVersion"`
	CoreID         string                           `json:"coreId"`
	CoreName       string                           `json:"coreName"`
	BrowserVersion string                           `json:"browserVersion"`
	BinarySHA256   string                           `json:"binarySha256,omitempty"`
	OS             string                           `json:"os"`
	Arch           string                           `json:"arch"`
	ConfigChannel  string                           `json:"configChannel"`
	GeneratedAt    string                           `json:"generatedAt"`
	Capabilities   []BrowserKernelCapabilityStatus  `json:"capabilities"`
	Warnings       []string                         `json:"warnings,omitempty"`
}

type kernelBinaryDigestCacheEntry struct {
	Size    int64
	ModTime int64
	SHA256  string
}

type BrowserKernelCapabilityStatus struct {
	Capability   string `json:"capability"`
	Configurable bool   `json:"configurable"`
	Emitted      bool   `json:"emitted"`
	Acknowledged bool   `json:"acknowledged"`
	Observed     bool   `json:"observed"`
	Verified     bool   `json:"verified"`
	Status       string `json:"status"`
	Note         string `json:"note,omitempty"`
}

func sha256RegularFile(path string) (string, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("kernel executable is not a regular file")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", nil, err
	}
	after, err := file.Stat()
	if err != nil {
		return "", nil, err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", nil, fmt.Errorf("kernel executable changed while hashing")
	}
	return hex.EncodeToString(digest.Sum(nil)), after, nil
}

func (a *App) kernelBinarySHA256(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("kernel executable is not a regular file")
	}
	key := strings.TrimSpace(path)
	a.kernelCapabilityMu.Lock()
	if cached, ok := a.kernelBinaryDigestCache[key]; ok &&
		cached.Size == info.Size() && cached.ModTime == info.ModTime().UnixNano() {
		a.kernelCapabilityMu.Unlock()
		return cached.SHA256, nil
	}
	a.kernelCapabilityMu.Unlock()

	digest, after, err := sha256RegularFile(path)
	if err != nil {
		return "", err
	}
	a.kernelCapabilityMu.Lock()
	if a.kernelBinaryDigestCache == nil {
		a.kernelBinaryDigestCache = make(map[string]kernelBinaryDigestCacheEntry)
	}
	a.kernelBinaryDigestCache[key] = kernelBinaryDigestCacheEntry{
		Size: after.Size(), ModTime: after.ModTime().UnixNano(), SHA256: digest,
	}
	a.kernelCapabilityMu.Unlock()
	return digest, nil
}

func (a *App) browserKernelCapabilityManifest(profile *BrowserProfile) BrowserKernelCapabilityManifest {
	manifest := BrowserKernelCapabilityManifest{
		SchemaVersion: 1,
		OS: runtime.GOOS,
		Arch: runtime.GOARCH,
		ConfigChannel: "launch-args-v1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Capabilities: []BrowserKernelCapabilityStatus{},
	}
	if a == nil || a.browserMgr == nil || profile == nil {
		manifest.Warnings = append(manifest.Warnings, "browser profile or manager is unavailable")
		return manifest
	}
	core, found := a.resolveBrowserCoreForFingerprintReport(profile.CoreId)
	if !found {
		manifest.Warnings = append(manifest.Warnings, "no usable core is configured")
		return manifest
	}
	manifest.CoreID = core.CoreId
	manifest.CoreName = core.CoreName
	manifest.BrowserVersion = a.browserMgr.GetChromeVersion(core.CorePath)
	if executable, err := a.browserMgr.ResolveCoreExecutable(core); err == nil {
		if digest, digestErr := a.kernelBinarySHA256(executable); digestErr == nil {
			manifest.BinarySHA256 = digest
		} else {
			manifest.Warnings = append(manifest.Warnings, "kernel binary hash unavailable: "+digestErr.Error())
		}
	} else {
		manifest.Warnings = append(manifest.Warnings, "kernel executable unavailable: "+err.Error())
	}

	report := a.buildBrowserFingerprintCapabilityReport(profile.ProfileId, core.CoreId, profile.FingerprintArgs)
	seen := make(map[string]struct{})
	for _, row := range report.Rows {
		key := strings.TrimSpace(row.Capability)
		if key == "" {
			continue
		}
		normalized := strings.ToLower(key)
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		emitted := strings.TrimSpace(row.RuntimeArg) != ""
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case "removed", "disabled", "not_effective", "pending", "overridden":
			emitted = false
		}
		status := "configurable"
		if emitted {
			status = "emitted"
		}
		manifest.Capabilities = append(manifest.Capabilities, BrowserKernelCapabilityStatus{
			Capability: key,
			Configurable: strings.TrimSpace(row.InputArg) != "" || emitted,
			Emitted: emitted,
			Acknowledged: false,
			Observed: false,
			Verified: false,
			Status: status,
			Note: row.Note,
		})
	}
	manifest.Capabilities = append(manifest.Capabilities,
		BrowserKernelCapabilityStatus{
			Capability: "managed-user-data-dir",
			Configurable: true, Emitted: true, Status: "emitted",
			Note: "application-owned launch setting; runtime acknowledgement is not independently probed",
		},
		BrowserKernelCapabilityStatus{
			Capability: "managed-debug-endpoint",
			Configurable: true, Emitted: true, Status: "emitted",
			Note: "application-owned launch setting; readiness is recorded in the launch receipt",
		},
		BrowserKernelCapabilityStatus{
			Capability: "managed-proxy",
			Configurable: true, Emitted: true, Status: "emitted",
			Note: "actual route evidence is tracked separately from kernel configuration",
		},
	)
	return manifest
}

// BrowserProfileKernelCapabilityManifest returns capability evidence for the
// actual configured/default core without claiming runtime verification.
func (a *App) BrowserProfileKernelCapabilityManifest(profileID string) (BrowserKernelCapabilityManifest, error) {
	if a == nil || a.browserMgr == nil {
		return BrowserKernelCapabilityManifest{}, fmt.Errorf("browser manager is unavailable")
	}
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	profile := a.browserMgr.Profiles[strings.TrimSpace(profileID)]
	if profile == nil {
		a.browserMgr.Mutex.Unlock()
		return BrowserKernelCapabilityManifest{}, fmt.Errorf("profile not found")
	}
	snapshot := copyBrowserProfileSnapshot(profile)
	a.browserMgr.Mutex.Unlock()
	return a.browserKernelCapabilityManifest(snapshot), nil
}
