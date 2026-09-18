package backend

import "testing"

func TestValidateBrowserStartURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com/path", true},
		{"http://127.0.0.1:9222/", true},
		{"about:blank", true},
		{"chrome://version", true},
		{"javascript:alert(1)", false},
		{"vbscript:msgbox(1)", false},
		{"data:text/html,<script>alert(1)</script>", false},
		{"file:///C:/Windows/System32/drivers/etc/hosts", false},
		{"example.com", false},
		{"https:///missing-host", false},
	}
	for _, test := range tests {
		err := ValidateBrowserStartURL(test.url)
		if test.valid && err != nil {
			t.Errorf("ValidateBrowserStartURL(%q) error = %v", test.url, err)
		}
		if !test.valid && err == nil {
			t.Errorf("ValidateBrowserStartURL(%q) returned nil, want rejection", test.url)
		}
	}
}
