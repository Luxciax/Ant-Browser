package logger

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var logURL = regexp.MustCompile(`(?i)(?:https?|socks5?|socks5h|vless|vmess|trojan|ss|ssr|hysteria2|hy2|tuic|anytls)://[^\s<>"']+`)
var logCredential = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|authorization|auth|api[_-]?key|access[_-]?key|private[_-]?key)\b["']?\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;&]+)`)

func redactLogText(text string) string {
	text = logURL.ReplaceAllStringFunc(text, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[REDACTED URL]"
		}
		switch strings.ToLower(u.Scheme) {
		case "vmess", "ss", "ssr":
			return u.Scheme + "://" + redacted
		}
		u.User = nil
		query := u.Query()
		for k := range query {
			if sensitiveLogKey(k) || strings.EqualFold(k, "key") {
				query.Set(k, redacted)
			}
		}
		u.RawQuery = query.Encode()
		return u.String()
	})
	return logCredential.ReplaceAllString(text, "$1="+redacted)
}

func sensitiveLogKey(key string) bool {
	normal := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	if normal == "auth" {
		return true
	}
	for _, part := range []string{"password", "passwd", "secret", "token", "authorization", "cookie", "privatekey", "apikey", "accesskey", "credential"} {
		if strings.Contains(normal, part) {
			return true
		}
	}
	return false
}

func redactLogValue(key string, value any, depth int) any {
	if sensitiveLogKey(key) {
		return redacted
	}
	if value == nil {
		return nil
	}
	if depth > 12 {
		return "[nested value omitted]"
	}
	if err, ok := value.(error); ok {
		return redactLogText(err.Error())
	}
	if text, ok := value.(string); ok {
		if strings.Contains(strings.ToLower(key), "proxy") && strings.Contains(strings.ToLower(key), "config") {
			text = strings.TrimSpace(text)
			match := logURL.FindStringIndex(text)
			if text != "direct://" && (match == nil || match[0] != 0) {
				return "[redacted proxy configuration]"
			}
		}
		return redactLogText(text)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return redactLogValue(key, v.String(), depth+1)
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return redactLogValue(key, v.Elem().Interface(), depth+1)
	case reflect.Map:
		out := map[string]any{}
		iter := v.MapRange()
		for iter.Next() {
			k := fmt.Sprint(iter.Key().Interface())
			out[k] = redactLogValue(k, iter.Value().Interface(), depth+1)
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = redactLogValue(key, v.Index(i).Interface(), depth+1)
		}
		return out
	case reflect.Struct:
		out := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.IsExported() {
				out[field.Name] = redactLogValue(field.Name, v.Field(i).Interface(), depth+1)
			}
		}
		return out
	}
	return value
}

func redactLogEntry(entry *LogEntry) *LogEntry {
	copyEntry := *entry
	copyEntry.Message = redactLogText(entry.Message)
	copyEntry.Error = redactLogText(entry.Error)
	if entry.Fields != nil {
		copyEntry.Fields = redactLogValue("", entry.Fields, 0).(map[string]any)
	}
	return &copyEntry
}
