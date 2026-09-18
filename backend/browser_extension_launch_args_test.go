package backend

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildBrowserLaunchArgsDoesNotLoadExtensionsFromStartupFlags(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, nil, false)
	if slices.Contains(args, "--load-extension") || slices.Contains(args, "--disable-extensions-except") {
		t.Fatalf("args = %#v, production extensions must not be loaded from startup flags", args)
	}
}

func TestBuildBrowserLaunchArgsLoadsPreparedExtensions(t *testing.T) {
	extensionDirs := []string{`C:\profiles\p1\Default\Extensions\aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\1.0.0_0`, `C:\profiles\p1\Default\Extensions\bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\2.0.0_0`}
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", extensionDirs, nil, nil, nil, nil, false)
	want := strings.Join(extensionDirs, ",")
	if !slices.Contains(args, "--load-extension="+want) {
		t.Fatalf("args = %#v, missing prepared extension load flag", args)
	}
	if slices.Contains(args, "--disable-extensions-except="+want) {
		t.Fatalf("args = %#v, prepared extensions must not disable profile-installed extensions", args)
	}
}
