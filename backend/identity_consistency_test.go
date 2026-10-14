package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/identity"
)

func TestIdentityFontPolicyStripsMetadataAndPreservesLegacy(t *testing.T) {
	base := []string{"--fingerprint-platform=macos", "--fingerprint=1", "--ant-identity-template=macos-4-1366x768", "--ant-geolocation=35,139,1000"}
	strict := buildBrowserFingerprintLaunchPlanForHost("a", append(append([]string(nil), base...), "--ant-font-policy=spoof"), "151.0.0.0", "windows")
	if strings.Contains(strings.Join(strict.launchArgs, "\n"), "--ant-") || strings.Contains(strings.Join(strict.launchArgs, "\n"), "--disable-spoofing=font") {
		t.Fatalf("font policy ignored: %v", strict.launchArgs)
	}
	legacy := buildBrowserFingerprintLaunchPlanForHost("a", base, "151.0.0.0", "windows")
	assertArgsContain(t, legacy.launchArgs, "--disable-spoofing=font")
	nativeHost := buildBrowserFingerprintLaunchPlanForHost("a", []string{"--fingerprint-platform=windows", "--ant-font-policy=host"}, "151.0.0.0", "windows")
	assertArgsContain(t, nativeHost.launchArgs, "--disable-spoofing=font")
}

func TestIdentityGenerateIsPreviewAndFindsDuplicateSeed(t *testing.T) {
	dir := t.TempDir()
	coreDir := filepath.Join(dir, "core")
	os.MkdirAll(coreDir, 0700)
	if err := os.WriteFile(filepath.Join(coreDir, "manifest.json"), []byte(`{"version":"151.0.7922.137"}`), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Browser: config.BrowserConfig{Cores: []config.BrowserCore{{CoreId: "core", CorePath: coreDir, IsDefault: true}}, DefaultCoreId: "core"}}
	m := browser.NewManager(cfg, dir)
	m.Profiles["a"] = &browser.Profile{ProfileId: "a", FingerprintArgs: []string{"--fingerprint=42"}}
	m.Profiles["b"] = &browser.Profile{ProfileId: "b", FingerprintArgs: []string{"--fingerprint=42"}}
	a := &App{browserMgr: m, config: cfg}
	result, err := a.BrowserIdentityGenerate(BrowserIdentityInput{ProfileID: "a", CoreID: "core", Args: []string{"--fingerprint=42"}, TemplateID: identity.Templates()[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Profiles["a"].FingerprintArgs, []string{"--fingerprint=42"}) {
		t.Fatal("preview persisted changes")
	}
	if len(result.Report.DuplicateProfiles) != 1 || result.Report.DuplicateProfiles[0] != "b" {
		t.Fatalf("duplicate seed not detected: %+v", result.Report)
	}
}

func TestBrowserProcessLanguageEnvIsPerProcess(t *testing.T) {
	base := []string{"PATH=keep", "LANGUAGE=zh_CN:zh", "LC_ALL=C.UTF-8", "UNRELATED=keep"}
	before := append([]string(nil), base...)
	got := browserProcessEnv(base, "linux", []string{"--lang=ja-JP"})
	if !strings.Contains(strings.Join(got, "\n"), "LANGUAGE=ja_JP:ja") || !strings.Contains(strings.Join(got, "\n"), "LC_ALL=C.UTF-8") || !reflect.DeepEqual(base, before) {
		t.Fatalf("environment leak: %v", got)
	}
	if !reflect.DeepEqual(browserProcessEnv(base, "windows", []string{"--lang=ja-JP"}), base) {
		t.Fatal("Windows environment changed")
	}
}

func TestPortableLoginRejectsAppBoundBeforeReadingLegacyKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Local State"), []byte(`{"os_crypt":{"encrypted_key":"invalid","app_bound_encrypted_key":"fixture-not-a-real-key"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAndUnprotectProfileOSCryptKey(dir); err != errPortableLoginAppBound {
		t.Fatalf("unsupported crypto reported as portable: %v", err)
	}
}

func TestFingerprintBaselineSeparatesCoreAndConfiguration(t *testing.T) {
	a := &App{}
	first, err := a.buildFingerprintCheckPageContextForExpectedArgsAndProfile("a", []string{"--fingerprint=42"}, &browser.Profile{CoreId: "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.buildFingerprintCheckPageContextForExpectedArgsAndProfile("a", []string{"--fingerprint=42"}, &browser.Profile{CoreId: "two"})
	if err != nil {
		t.Fatal(err)
	}
	var one, two fingerprintCheckPageContext
	json.Unmarshal(first, &one)
	json.Unmarshal(second, &two)
	if one.ConfigKey == two.ConfigKey {
		t.Fatal("core change reused baseline")
	}
	third, _ := a.buildFingerprintCheckPageContextForExpectedArgsAndProfile("a", []string{"--fingerprint=42", "--lang=ja-JP"}, &browser.Profile{CoreId: "one"})
	json.Unmarshal(third, &two)
	if one.ConfigKey == two.ConfigKey {
		t.Fatal("config change reused baseline")
	}
	geo, _ := a.buildFingerprintCheckPageContextForExpectedArgsAndProfile("a", []string{"--fingerprint=42"}, &browser.Profile{CoreId: "one", FingerprintArgs: []string{"--ant-geolocation=35,139,1000"}})
	json.Unmarshal(geo, &two)
	if one.ConfigKey == two.ConfigKey {
		t.Fatal("CDP geolocation metadata change reused baseline")
	}
	if !strings.Contains(fingerprintCheckHTML, "visibilitychange") || !strings.Contains(fingerprintCheckHTML, "baseline:v4:") {
		t.Fatal("foreground/versioned baseline missing")
	}
}

func TestWindowSizeSyncPreservesStateAndDeduplicatesWindows(t *testing.T) {
	for _, state := range []string{"normal", "maximized", "fullscreen", "minimized"} {
		sets := 0
		width, height := float64(800), float64(600)
		call := func(_ int, method string, params map[string]any) (map[string]any, error) {
			switch method {
			case "Target.getTargets":
				return map[string]any{"targetInfos": []any{map[string]any{"type": "page", "targetId": "a"}, map[string]any{"type": "page", "targetId": "b"}}}, nil
			case "Browser.getWindowForTarget":
				return map[string]any{"windowId": float64(1), "bounds": map[string]any{"width": width, "height": height, "windowState": state}}, nil
			case "Browser.setWindowBounds":
				sets++
				b := params["bounds"].(map[string]any)
				width = float64(b["width"].(int))
				height = float64(b["height"].(int))
				return map[string]any{}, nil
			}
			t.Fatalf("unexpected command %s", method)
			return nil, nil
		}
		if err := syncConfiguredBrowserWindowSizeWithCall(1, []string{"--window-size=1366,768"}, call); err != nil {
			t.Fatal(err)
		}
		if state == "normal" && sets != 1 || state != "normal" && sets != 0 {
			t.Fatalf("window state altered: %s %d", state, sets)
		}
	}
}
