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
	Mode               string `json:"mode"`
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

func (clearcoteKernelAdapter) launchPlan(id string, args []string, _ string) browserFingerprintLaunchPlan {
	stored, err := readClearcoteStoredConfig(args)
	plan := browserFingerprintLaunchPlan{launchArgs: []string{}, rows: []BrowserFingerprintCapabilityRow{}, warnings: []string{}}
	if err != nil {
		plan.warnings = append(plan.warnings, err.Error())
		return plan
	}
	args = normalizeBrowserLanguageArgs(args)
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
