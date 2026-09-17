package backend

import (
	"slices"
	"testing"
)

func TestBuildBrowserLaunchArgsDoesNotLoadExtensionsFromStartupFlags(t *testing.T) {
	args := buildBrowserLaunchArgs("profile-dir", 9222, "direct://", nil, nil, nil, nil, false)
	if slices.Contains(args, "--load-extension") || slices.Contains(args, "--disable-extensions-except") {
		t.Fatalf("args = %#v, production extensions must not be loaded from startup flags", args)
	}
}

func TestBuildBrowserExtensionInstallerLaunchArgsNeverRestoresSession(t *testing.T) {
	profileArgs, _ := sanitizeManagedLaunchArgs([]string{"--restore-last-session", "--disable-sync", "--new-window", "--kiosk", "--app=https://example.com"})
	args := buildBrowserExtensionInstallerLaunchArgs("profile-dir", 9222, "direct://", []string{"--start-maximized"}, profileArgs, nil)
	if slices.Contains(args, "--restore-last-session") {
		t.Fatalf("args = %#v, extension installer must not restore the user session", args)
	}
	for _, forbidden := range []string{"--new-window", "--kiosk", "--start-maximized", "--app=https://example.com"} {
		if slices.Contains(args, forbidden) {
			t.Fatalf("args = %#v, extension installer must not inherit window-opening arg %q", args, forbidden)
		}
	}
	if !slices.Contains(args, "--remote-debugging-port=9222") {
		t.Fatalf("args = %#v, extension installer must keep the assigned debugging port", args)
	}
}
