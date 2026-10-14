package identity

import (
	"reflect"
	"strings"
	"testing"
)

func TestTemplatesPreserveSeedAndUnknownArgs(t *testing.T) {
	if len(Templates()) != 12 {
		t.Fatal("expected compact 12-device pool")
	}
	for _, template := range Templates() {
		before := []string{"--fingerprint=12345", "--timezone=Asia/Tokyo", "--lang=ja-JP", "--accept-lang=ja-JP,ja", "--disable-non-proxied-udp", "--custom-user-flag=keep"}
		args, err := ApplyTemplate(before, template.ID, "999", "151.0.7922.137")
		if err != nil {
			t.Fatal(err)
		}
		id := FromArgs(args)
		if id.Seed != "12345" || id.Timezone != "Asia/Tokyo" || id.Locale != "ja-JP" || !strings.Contains(strings.Join(args, "\n"), "--custom-user-flag=keep") {
			t.Fatalf("identity unexpectedly changed: %+v", id)
		}
		if id.BrandVersion != "151.0.7922.137" || !strings.Contains(id.UserAgent, "Chrome/151.0.0.0") {
			t.Fatalf("versions not aligned: %+v", id)
		}
		if template.Platform == "linux" && (strings.Contains(id.UserAgent, "Windows") || !strings.Contains(id.UserAgent, "Linux")) {
			t.Fatal("Linux UA regressed to Windows")
		}
		if issues := ValidateArgs(args, "151.0.7922.137"); len(issues) != 0 {
			t.Fatalf("template conflicts: %+v", issues)
		}
		again, err := ApplyTemplate(args, template.ID, "999", "151.0.7922.137")
		if err != nil || !reflect.DeepEqual(args, again) {
			t.Fatalf("template not stable: %v", err)
		}
	}
}

func TestIdentityHashIgnoresTemplateProvenance(t *testing.T) {
	a := Identity{Seed: "42", Platform: "linux", TemplateID: "one"}
	b := a
	b.TemplateID = "two"
	if a.Hash() != b.Hash() {
		t.Fatal("provenance makes identical devices appear unique")
	}
	b.Seed = "43"
	if a.Hash() == b.Hash() {
		t.Fatal("seed not included")
	}
}

func TestIdentityValidationRejectsInvalidInputs(t *testing.T) {
	for _, arg := range []string{"--fingerprint=0", "--fingerprint=4294967295", "--fingerprint-hardware-concurrency=zero", "--timezone=Not/AZone", "--window-size=0,100", "--ant-geolocation=nan,1,1", "--ant-geolocation=oops", "--ant-font-policy=unknown"} {
		found := false
		for _, issue := range ValidateArgs([]string{arg}, "151.0.0.0") {
			if issue.Level == "error" {
				found = true
			}
		}
		if !found {
			t.Errorf("not rejected: %s", arg)
		}
	}
}

func TestIdentityWarnsAboutInconsistentVersionsAndLocale(t *testing.T) {
	issues := ValidateArgs([]string{"--fingerprint-platform=linux", "--user-agent=Mozilla Windows NT Chrome/120.0.0.0", "--fingerprint-brand-version=148.0.0.0", "--lang=ja-JP", "--accept-lang=en-US,en"}, "151.0.0.0")
	if len(issues) != 4 {
		t.Fatalf("expected four contradictions: %+v", issues)
	}
}

func TestTemplateRejectsUnknownKernelWithoutMutatingInput(t *testing.T) {
	args := []string{"--fingerprint=7", "--unknown=keep"}
	before := append([]string(nil), args...)
	if _, err := ApplyTemplate(args, Templates()[0].ID, "", ""); err == nil {
		t.Fatal("unknown kernel accepted")
	}
	if !reflect.DeepEqual(args, before) {
		t.Fatal("input mutated")
	}
}
