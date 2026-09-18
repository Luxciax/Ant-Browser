package backend

import (
	"fmt"
	"net/url"
	"strings"
)

var allowedBrowserStartURLSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
	"about": {},
	"chrome": {},
}

func ValidateBrowserStartURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("启动地址不能为空")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("启动地址格式无效: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if _, ok := allowedBrowserStartURLSchemes[scheme]; !ok {
		return fmt.Errorf("不允许的启动地址协议 %q，仅支持 http/https/about/chrome", scheme)
	}
	if (scheme == "http" || scheme == "https") && strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("%s 启动地址缺少主机名", scheme)
	}
	return nil
}

func validateBrowserStartURLs(items []string) error {
	for _, item := range normalizeNonEmptyStrings(items) {
		if err := ValidateBrowserStartURL(item); err != nil {
			return fmt.Errorf("启动地址 %q 无效: %w", item, err)
		}
	}
	return nil
}
