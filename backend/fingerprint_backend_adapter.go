package backend

import (
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

const clearcoteConfigArg = "--ant-clearcote-config="

type clearcoteStoredConfig struct {
	Values             map[string]json.RawMessage `json:"-"`
	Mode               string                     `json:"mode"`
	FingerprintProfile *struct {
		Source   string `json:"source"`
		FilePath string `json:"filePath"`
	} `json:"fingerprintProfile,omitempty"`
}

func readClearcoteStoredConfig(args []string) (clearcoteStoredConfig, error) {
	result := clearcoteStoredConfig{Mode: "auto"}
	for _, arg := range args {
		if strings.HasPrefix(arg, clearcoteConfigArg) {
			result = clearcoteStoredConfig{Mode: "auto"}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, clearcoteConfigArg)), &result); err != nil {
				return result, fmt.Errorf("Clearcote 配置无效: %w", err)
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(arg, clearcoteConfigArg)), &result.Values); err != nil {
				return result, err
			}
		}
	}
	if result.Mode != "auto" && result.Mode != "profile" && result.Mode != "manual" {
		return result, fmt.Errorf("未知 Clearcote 模式: %s", result.Mode)
	}
	return result, nil
}

type kernelFingerprintAdapter interface {
	launchPlan(profileID string, args []string, version string) browserFingerprintLaunchPlan
	expected(args []string) BrowserFingerprintExpectedInfo
}

type fingerprintChromiumKernelAdapter struct{}

func (fingerprintChromiumKernelAdapter) launchPlan(id string, args []string, version string) browserFingerprintLaunchPlan {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		key := browserFingerprintArgKey(arg)
		if strings.HasPrefix(key, "--ant-") || key == "--fingerprint-profile" || key == "--fingerprint-max-touch-points" || key == "--fingerprint-device-pixel-ratio" || key == "--fingerprint-avail-width" || key == "--fingerprint-avail-height" || key == "--webrtc-ip" {
			continue
		}
		filtered = append(filtered, arg)
	}
	return buildBrowserFingerprintLaunchPlan(id, filtered, version)
}
func (fingerprintChromiumKernelAdapter) expected(args []string) BrowserFingerprintExpectedInfo {
	return buildBrowserFingerprintExpected(args)
}

type genericChromiumKernelAdapter struct{}

func (genericChromiumKernelAdapter) launchPlan(_ string, args []string, _ string) browserFingerprintLaunchPlan {
	plan := browserFingerprintLaunchPlan{launchArgs: []string{}, rows: []BrowserFingerprintCapabilityRow{}, warnings: []string{"普通 Chromium 不提供原生指纹伪装"}}
	for _, arg := range normalizeBrowserLanguageArgs(args) {
		key := browserFingerprintArgKey(arg)
		if strings.HasPrefix(key, "--ant-") || strings.HasPrefix(key, "--fingerprint") || key == "--disable-spoofing" || key == "--disable-gpu-fingerprint" || key == "--timezone" || key == "--webrtc-ip" {
			continue
		}
		plan.launchArgs = append(plan.launchArgs, arg)
	}
	return plan
}
func (genericChromiumKernelAdapter) expected(args []string) BrowserFingerprintExpectedInfo {
	return buildBrowserFingerprintExpected(args)
}

type clearcoteKernelAdapter struct{}

var clearcoteNativeArgLabels = map[string]string{
	"--fingerprint": "指纹种子", "--fingerprint-platform": "平台", "--fingerprint-platform-version": "系统版本",
	"--fingerprint-brand": "品牌", "--fingerprint-brand-version": "品牌版本",
	"--fingerprint-hardware-concurrency": "CPU", "--fingerprint-device-memory": "RAM",
	"--fingerprint-screen-width": "屏幕宽度", "--fingerprint-screen-height": "屏幕高度",
	"--fingerprint-avail-width": "可用屏幕宽度", "--fingerprint-avail-height": "可用屏幕高度",
	"--fingerprint-color-depth": "色深", "--fingerprint-device-pixel-ratio": "DPR", "--fingerprint-max-touch-points": "触控点",
	"--fingerprint-gpu-vendor": "GPU Vendor", "--fingerprint-gpu-renderer": "GPU Renderer",
	"--fingerprint-location": "地理位置", "--fingerprint-profile": "真实设备 Profile", "--webrtc-ip": "WebRTC IP",
	"--lang": "语言", "--accept-lang": "语言列表", "--timezone": "时区", "--window-size": "窗口",
	"--disable-non-proxied-udp": "WebRTC", "--webrtc-ip-handling-policy": "WebRTC",
}
var clearcoteManualArgs = map[string]bool{
	"--fingerprint-hardware-concurrency": true, "--fingerprint-device-memory": true,
	"--fingerprint-screen-width": true, "--fingerprint-screen-height": true, "--fingerprint-avail-width": true, "--fingerprint-avail-height": true,
	"--fingerprint-color-depth": true, "--fingerprint-device-pixel-ratio": true, "--fingerprint-max-touch-points": true,
	"--fingerprint-gpu-vendor": true, "--fingerprint-gpu-renderer": true,
}

// Restore dormant configuration after editing with another backend. Explicit raw
// switches win; metadata only fills fields that the other serializer cannot model.
func restoreClearcoteArgs(args []string, stored clearcoteStoredConfig) []string {
	result := append([]string{}, args...)
	fields := [][2]string{
		{"seed", "fingerprint"}, {"platform", "fingerprint-platform"}, {"platformVersion", "fingerprint-platform-version"},
		{"brand", "fingerprint-brand"}, {"brandVersion", "fingerprint-brand-version"},
		{"hardwareConcurrency", "fingerprint-hardware-concurrency"}, {"deviceMemory", "fingerprint-device-memory"},
		{"screenWidth", "fingerprint-screen-width"}, {"screenHeight", "fingerprint-screen-height"},
		{"availWidth", "fingerprint-avail-width"}, {"availHeight", "fingerprint-avail-height"},
		{"colorDepth", "fingerprint-color-depth"}, {"devicePixelRatio", "fingerprint-device-pixel-ratio"}, {"touchPoints", "fingerprint-max-touch-points"},
		{"gpuVendor", "fingerprint-gpu-vendor"}, {"gpuRenderer", "fingerprint-gpu-renderer"},
		{"location", "fingerprint-location"}, {"webrtcIp", "webrtc-ip"},
	}
	for _, field := range fields {
		flag := "--" + field[1]
		if browserLastArgIndexWithKey(result, flag) >= 0 {
			continue
		}
		var value string
		if json.Unmarshal(stored.Values[field[0]], &value) == nil && value != "" {
			result = append(result, flag+"="+value)
		}
	}
	return result
}

func (clearcoteKernelAdapter) launchPlan(id string, args []string, _ string) browserFingerprintLaunchPlan {
	stored, err := readClearcoteStoredConfig(args)
	plan := browserFingerprintLaunchPlan{launchArgs: []string{}, rows: []BrowserFingerprintCapabilityRow{}, warnings: []string{}}
	if err != nil {
		plan.warnings = append(plan.warnings, err.Error())
		return plan
	}
	args = normalizeBrowserLanguageArgs(restoreClearcoteArgs(args, stored))
	if !browserArgsHaveFingerprintSeed(args) && id != "" {
		plan.launchArgs = append(plan.launchArgs, fmt.Sprintf("--fingerprint=%d", browserFingerprintSeedForProfileID(id)))
	}
	if browserArgValue(args, "--fingerprint-brand") == "" {
		plan.launchArgs = append(plan.launchArgs, "--fingerprint-brand=Chrome")
	}
	if browserArgValue(args, "--fingerprint-platform") == "" {
		plan.launchArgs = append(plan.launchArgs, "--fingerprint-platform="+normalizeFingerprintPlatform(runtime.GOOS))
	}
	for index, arg := range args {
		key := browserFingerprintArgKey(arg)
		if strings.HasPrefix(key, "--ant-") {
			continue
		}
		label, known := clearcoteNativeArgLabels[key]
		if clearcoteManualArgs[key] && stored.Mode != "manual" {
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{Capability: label, Status: "disabled", InputArg: arg, Action: "Persona 管理", Note: "仅高级手动模式输出覆盖参数"})
			continue
		}
		if key == "--fingerprint-profile" && stored.Mode == "auto" {
			continue
		}
		if browserLastArgIndexWithKey(args, key) != index {
			continue
		}
		if !known && (strings.HasPrefix(key, "--fingerprint") || key == "--disable-spoofing" || key == "--disable-gpu-fingerprint") {
			continue
		}
		plan.launchArgs = append(plan.launchArgs, arg)
		if known {
			plan.rows = append(plan.rows, BrowserFingerprintCapabilityRow{Capability: label, Status: "kept", InputArg: arg, RuntimeArg: arg, Action: "Clearcote 原生参数", Note: "由 Clearcote Adapter 传递；以当前内核运行检测为准"})
		}
	}
	return plan
}
func (clearcoteKernelAdapter) expected(args []string) BrowserFingerprintExpectedInfo {
	result := buildBrowserFingerprintExpected(args)
	result.ScreenWidth = browserArgValue(args, "--fingerprint-screen-width")
	result.ScreenHeight = browserArgValue(args, "--fingerprint-screen-height")
	result.AvailWidth = browserArgValue(args, "--fingerprint-avail-width")
	result.AvailHeight = browserArgValue(args, "--fingerprint-avail-height")
	result.DevicePixelRatio = browserArgValue(args, "--fingerprint-device-pixel-ratio")
	result.DeviceMemory = browserArgValue(args, "--fingerprint-device-memory")
	result.ColorDepth = browserArgValue(args, "--fingerprint-color-depth")
	result.TouchPoints = browserArgValue(args, "--fingerprint-max-touch-points")
	result.WebGLVendor = browserArgValue(args, "--fingerprint-gpu-vendor")
	result.WebGLRenderer = browserArgValue(args, "--fingerprint-gpu-renderer")
	return result
}

func fingerprintAdapterForBackend(backend config.FingerprintBackend) kernelFingerprintAdapter {
	switch backend.LegacyDefault() {
	case config.FingerprintBackendClearcote:
		return clearcoteKernelAdapter{}
	case config.FingerprintBackendGenericChromium:
		return genericChromiumKernelAdapter{}
	default:
		return fingerprintChromiumKernelAdapter{}
	}
}

func (a *App) fingerprintExpectedForProfile(id string, profile *BrowserProfile, args []string) BrowserFingerprintExpectedInfo {
	coreID := ""
	if profile == nil && id != "" && a != nil && a.browserMgr != nil {
		profile, _ = a.fingerprintCheckProfileSnapshot(id)
	}
	if profile != nil {
		coreID = profile.CoreId
	}
	core, _ := a.resolveBrowserCoreForFingerprintReport(coreID)
	return fingerprintAdapterForBackend(core.FingerprintBackend).expected(args)
}
