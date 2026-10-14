package logger

import (
	"errors"
	"strings"
	"testing"
)

func TestAuditReviewProxyCredentialRedaction(t *testing.T) {
	memory := &MemoryWriter{maxSize: 10}
	log := &Logger{component: "Browser", level: INFO, writers: []Writer{memory}}
	// Fixed synthetic values only; no saved proxy configuration is accessed.
	log.Info("代理配置检查", F("resolved_proxy_config", "socks5://test-user:TEST_ONLY_FAKE_PASSWORD@127.0.0.1:1080"))
	entries := memory.GetEntries()
	if len(entries) != 1 {
		t.Fatal("test logger did not write")
	}
	raw, _ := entries[0].Fields["resolved_proxy_config"].(string)
	if strings.Contains(raw, "TEST_ONLY_FAKE_PASSWORD") {
		t.Fatal("INFO logger retained the complete synthetic proxy password in the frontend log buffer")
	}
}

func TestRedactionCoversNestedValuesAndErrorsWithoutMutatingCaller(t *testing.T) {
	fake := "TEST_ONLY_PRIVATE_VALUE"
	fields := map[string]any{"items": []any{map[string]any{"password": fake, "endpoint": "socks5://user:" + fake + "@localhost:1080"}}, "failure": errors.New("connect trojan://" + fake + "@localhost:443 failed")}
	entry := NewLogEntry(ERROR, "test", "request failed password="+fake)
	entry.Fields = fields
	entry.Error = "vmess://" + fake
	clean := redactLogEntry(entry)
	data, err := NewJSONFormatter().Format(clean)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), fake) {
		t.Fatal("nested or error credentials leaked")
	}
	if fields["items"].([]any)[0].(map[string]any)["password"] != fake {
		t.Fatal("redaction mutated input")
	}
	if !strings.Contains(string(data), "localhost:1080") {
		t.Fatal("lost diagnostic endpoint")
	}
}
