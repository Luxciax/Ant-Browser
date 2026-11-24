package browser

import "testing"

func TestPhase5CompatibilityTemplatesDeclareKernelRequirements(t *testing.T) {
	ids := []string{
		"desktop-windows-compatible",
		"desktop-macos-compatible",
		"desktop-linux-compatible",
		"mobile-android-compatible",
		"mobile-ios-compatible",
	}
	for _, id := range ids {
		template, ok := FindEnvironmentTemplate(id)
		if !ok {
			t.Fatalf("missing compatibility template %q", id)
		}
		if template.SupportLevel != "requires-compatible-kernel" {
			t.Fatalf("%s support level = %q", id, template.SupportLevel)
		}
		if template.TargetOS == "" || template.DeviceClass == "" || template.KernelFamily == "" {
			t.Fatalf("%s missing compatibility metadata: %#v", id, template)
		}
		if len(template.RequiredCapabilities) == 0 {
			t.Fatalf("%s must declare required capabilities", id)
		}
	}
}

func TestBuiltinEnvironmentTemplatesReturnDefensiveCopies(t *testing.T) {
	first := BuiltinEnvironmentTemplates()
	if len(first) == 0 {
		t.Fatal("expected builtin templates")
	}
	first[0].Rules = nil
	first[0].RequiredCapabilities = []string{"mutated"}

	second := BuiltinEnvironmentTemplates()
	if len(second[0].Rules) == 0 {
		t.Fatal("template rules leaked mutable caller state")
	}
	if len(second[0].RequiredCapabilities) == 1 && second[0].RequiredCapabilities[0] == "mutated" {
		t.Fatal("required capabilities leaked mutable caller state")
	}
}
