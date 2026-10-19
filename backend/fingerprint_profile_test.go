package backend

import (
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFingerprintProfileEncodingAndModes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	raw := `{"navigator":{"deviceMemory":4,"languages":["en-US","en"]},"screen":{"width":1920}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	payload, err := encodeFingerprintProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil || string(decoded) != raw {
		t.Fatalf("invalid SDK payload: %s %v", decoded, err)
	}
	for _, invalid := range []string{`[]`, `null`, `not JSON`} {
		if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := encodeFingerprintProfile(path); err == nil {
			t.Fatalf("accepted invalid JSON: %s", invalid)
		}
	}
	args, err := prepareClearcoteProfileArgs([]string{clearcoteConfigArg + `{"mode":"auto","fingerprintProfile":{"source":"file","filePath":"missing.json"}}`}, []string{"--fingerprint=1"})
	if err != nil || len(args) != 1 {
		t.Fatalf("auto should ignore dormant Profile: %v %v", args, err)
	}
	if _, err := prepareClearcoteProfileArgs([]string{clearcoteConfigArg + `{"mode":"profile"}`}, nil); err == nil {
		t.Fatal("Profile mode must require a file")
	}
}

func TestFingerprintProfileImportOwnsCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source.json")
	raw := []byte(`{"navigator":{"platform":"Win32"}}`)
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	app := &App{appRoot: root}
	result, err := app.importFingerprintProfileFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if result.FilePath == source {
		t.Fatal("import must copy into managed storage")
	}
	// Moving the user's original must not break launch-time encoding.
	if err := os.Rename(source, source+".moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := encodeFingerprintProfile(result.FilePath); err != nil {
		t.Fatal(err)
	}
}
