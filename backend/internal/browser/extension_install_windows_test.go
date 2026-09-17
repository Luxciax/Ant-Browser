//go:build windows
// +build windows

package browser

import (
	"slices"
	"testing"
)

func TestHasRemoteDebuggingPortArg(t *testing.T) {
	if !hasRemoteDebuggingPortArg([]string{"--no-first-run", "--remote-debugging-port=9222"}) {
		t.Fatal("expected remote debugging port to be detected")
	}
	if hasRemoteDebuggingPortArg([]string{"--no-first-run", "about:blank"}) {
		t.Fatal("unexpected remote debugging port detection")
	}
}

func TestAppendExtensionInstallerStartupArgsUsesNoStartupWindowWithDebugPort(t *testing.T) {
	args := appendExtensionInstallerStartupArgs([]string{"--remote-debugging-port=9222"})
	if !slices.Contains(args, "--no-startup-window") {
		t.Fatalf("args = %#v, want --no-startup-window", args)
	}
	if slices.Contains(args, "about:blank") {
		t.Fatalf("args = %#v, debug-port installer must not create an about:blank window", args)
	}
}

func TestAppendExtensionInstallerStartupArgsFallsBackToBlankWithoutDebugPort(t *testing.T) {
	args := appendExtensionInstallerStartupArgs([]string{"--no-first-run"})
	if !slices.Contains(args, "about:blank") {
		t.Fatalf("args = %#v, want compatibility fallback about:blank", args)
	}
}
