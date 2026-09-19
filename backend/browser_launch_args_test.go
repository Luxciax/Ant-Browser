package backend

import (
	"reflect"
	"testing"
)

func TestSanitizeManagedLaunchArgsRejectsDangerousFlags(t *testing.T) {
	input := []string{
		"--proxy-pac-url=http://127.0.0.1/proxy.pac",
		"--renderer-cmd-prefix", "cmd.exe /c calc",
		"--utility-cmd-prefix=evil",
		"--gpu-launcher", "evil-gpu",
		"--browser-subprocess-path=C:\\evil.exe",
		"--no-sandbox",
		"--disable-sandbox",
		"--single-process",
		"--remote-allow-origins=*",
		"--disable-gpu",
	}

	got, removed := sanitizeManagedLaunchArgs(input)
	if want := []string{"--disable-gpu"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sanitized args = %#v, want %#v", got, want)
	}

	for _, denied := range []string{
		"--proxy-pac-url",
		"--renderer-cmd-prefix",
		"--utility-cmd-prefix",
		"--gpu-launcher",
		"--browser-subprocess-path",
		"--no-sandbox",
		"--disable-sandbox",
		"--single-process",
		"--remote-allow-origins",
	} {
		found := false
		for _, item := range removed {
			if item == denied {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("removed args = %#v, missing %s", removed, denied)
		}
	}
}
