package logger

import "testing"

func TestMethodInterceptorMasksSensitiveMapFieldNamesBySubstring(t *testing.T) {
	interceptor := NewMethodInterceptor(nil, InterceptorConfig{
		SensitiveFields: []string{"password", "token", "secret"},
	})

	masked, ok := interceptor.maskValue(map[string]string{
		"migrationPassword": "portable-login-passphrase",
		"apiToken":          "api-token-value",
		"clientSecret":      "client-secret-value",
		"profileName":       "Profile A",
	}).(map[string]interface{})
	if !ok {
		t.Fatalf("masked map type = %T, want map[string]interface{}", interceptor.maskValue(map[string]string{}))
	}

	for _, key := range []string{"migrationPassword", "apiToken", "clientSecret"} {
		if got := masked[key]; got != "***" {
			t.Fatalf("%s = %#v, want masked value", key, got)
		}
	}
	if got := masked["profileName"]; got != "Profile A" {
		t.Fatalf("profileName = %#v, want original value", got)
	}
}

func TestMethodInterceptorMasksSensitiveStructFieldNamesBySubstring(t *testing.T) {
	type request struct {
		MigrationPassword string
		AccessToken       string
		ClientSecret      string
		ProfileName       string
	}

	interceptor := NewMethodInterceptor(nil, InterceptorConfig{
		SensitiveFields: []string{"password", "token", "secret"},
	})
	masked, ok := interceptor.maskValue(request{
		MigrationPassword: "portable-login-passphrase",
		AccessToken:       "access-token-value",
		ClientSecret:      "client-secret-value",
		ProfileName:       "Profile B",
	}).(map[string]interface{})
	if !ok {
		t.Fatalf("masked struct type is unexpected")
	}

	for _, key := range []string{"MigrationPassword", "AccessToken", "ClientSecret"} {
		if got := masked[key]; got != "***" {
			t.Fatalf("%s = %#v, want masked value", key, got)
		}
	}
	if got := masked["ProfileName"]; got != "Profile B" {
		t.Fatalf("ProfileName = %#v, want original value", got)
	}
}
