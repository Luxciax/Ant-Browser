package backend

import (
	"ant-chrome/backend/internal/logger"
	"strings"
)

type LaunchArgPolicy struct {
	DeniedExactPrefixes []string
	DeniedExactNames    []string
	AllowedInternal     []string
}

var browserLaunchArgPolicy = LaunchArgPolicy{
	DeniedExactPrefixes: []string{
		"--user-data-dir",
		"--remote-debugging-port",
		"--remote-debugging-address",
		"--proxy-server",
		"--proxy-pac-url",
		"--load-extension",
		"--disable-extensions-except",
		"--renderer-cmd-prefix",
		"--utility-cmd-prefix",
		"--gpu-launcher",
		"--browser-subprocess-path",
		"--remote-allow-origins",
	},
	DeniedExactNames: []string{
		"--remote-debugging-pipe",
		"--no-sandbox",
		"--disable-sandbox",
		"--single-process",
		"--restore-last-session",
	},
	AllowedInternal: []string{
		"--user-data-dir",
		"--remote-debugging-port",
		"--proxy-server",
		"--load-extension",
		"--disable-extensions-except",
		"--restore-last-session",
	},
}

func sanitizeManagedLaunchArgs(args []string) ([]string, []string) {
	return browserLaunchArgPolicy.Sanitize(args)
}

func (p LaunchArgPolicy) Sanitize(args []string) ([]string, []string) {
	if len(args) == 0 {
		return nil, nil
	}

	sanitized := make([]string, 0, len(args))
	removed := make([]string, 0, 4)

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}

		name, takesValue, matched := p.matchDenied(arg)
		if !matched {
			sanitized = append(sanitized, arg)
			continue
		}

		removed = appendUniqueString(removed, name)
		if takesValue && !strings.Contains(arg, "=") && i+1 < len(args) {
			next := strings.TrimSpace(args[i+1])
			if next != "" && !strings.HasPrefix(next, "-") {
				i++
			}
		}
	}

	return sanitized, removed
}

func (p LaunchArgPolicy) matchDenied(arg string) (string, bool, bool) {
	for _, prefix := range p.DeniedExactPrefixes {
		if launchArgNameMatches(arg, prefix) {
			return prefix, true, true
		}
	}
	for _, name := range p.DeniedExactNames {
		if launchArgNameMatches(arg, name) {
			return name, false, true
		}
	}
	return "", false, false
}

func launchArgNameMatches(arg string, name string) bool {
	arg = strings.TrimSpace(arg)
	name = strings.TrimSpace(name)
	if arg == "" || name == "" {
		return false
	}
	return strings.EqualFold(arg, name) || strings.HasPrefix(strings.ToLower(arg), strings.ToLower(name)+"=")
}

func logManagedLaunchArgOverrides(log *logger.Logger, profileId string, source string, managedArgs []string) {
	if log == nil || len(managedArgs) == 0 {
		return
	}
	log.Warn("忽略由系统接管的浏览器启动参数",
		logger.F("profile_id", profileId),
		logger.F("source", source),
		logger.F("managed_args", managedArgs),
	)
}

func appendUniqueString(items []string, value string) []string {
	for _, item := range items {
		if strings.EqualFold(item, value) {
			return items
		}
	}
	return append(items, value)
}
