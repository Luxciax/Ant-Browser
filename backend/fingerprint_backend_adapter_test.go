package backend

import (
	"ant-chrome/backend/internal/config"
	"reflect"
	"strings"
	"testing"
)

func TestFingerprintBackendIsolation(t *testing.T) {
	args := []string{"--fingerprint=12345", "--fingerprint-platform=windows", "--fingerprint-device-memory=4", "--fingerprint-screen-width=1920", "--fingerprint-max-touch-points=7", "--fingerprint-gpu-vendor=Example", clearcoteConfigArg + `{"mode":"manual"}`}
	clear := fingerprintAdapterForBackend(config.FingerprintBackendClearcote).launchPlan("profile", args, "150.0.0.0")
	for _, arg := range args[:len(args)-1] {
		if !hasStringValue(clear.launchArgs, arg) {
			t.Fatalf("Clearcote removed %s: %v", arg, clear.launchArgs)
		}
	}
	legacy := fingerprintAdapterForBackend(config.FingerprintBackendFingerprintChromium).launchPlan("profile", args, "150.0.0.0")
	for _, arg := range legacy.launchArgs {
		if strings.Contains(arg, "device-memory") || strings.Contains(arg, "max-touch-points") || strings.HasPrefix(arg, "--ant-") {
			t.Fatalf("Clearcote argument leaked: %s", arg)
		}
	}
	generic := fingerprintAdapterForBackend(config.FingerprintBackendGenericChromium).launchPlan("profile", args, "150")
	if len(generic.launchArgs) != 0 {
		t.Fatalf("generic must not receive fingerprint args: %v", generic.launchArgs)
	}
}

func TestClearcoteAutoPersonaAndStableSeed(t *testing.T) {
	adapter := clearcoteKernelAdapter{}
	raw := []string{"--fingerprint-hardware-concurrency=8", "--fingerprint-device-memory=4", "--fingerprint-screen-width=1920"}
	plan := adapter.launchPlan("profile", raw, "150")
	for _, arg := range plan.launchArgs {
		if clearcoteManualArgs[browserFingerprintArgKey(arg)] {
			t.Fatalf("automatic mode overrides Persona: %s", arg)
		}
	}
	if !reflect.DeepEqual(plan.launchArgs, adapter.launchPlan("profile", raw, "150").launchArgs) {
		t.Fatal("seed changed between launches")
	}
	if reflect.DeepEqual(plan.launchArgs, adapter.launchPlan("another", raw, "150").launchArgs) {
		t.Fatal("new instances need distinct seeds")
	}
}

func TestFingerprintChromiumAdapterPreservesLegacyLaunchPlan(t *testing.T) {
	args := []string{"--fingerprint=1234", "--fingerprint-platform=macos", "--fingerprint-device-memory=8", "--disable-gpu-fingerprint", "--fingerprint-canvas-noise=1", "--lang=ja-JP", "--custom=value"}
	for _, version := range []string{"", "143.0.0.0", "144.0.0.0", "150.0.0.0"} {
		want := buildBrowserFingerprintLaunchPlan("profile", args, version)
		got := fingerprintChromiumKernelAdapter{}.launchPlan("profile", args, version)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("legacy behavior changed for %s", version)
		}
	}
}
