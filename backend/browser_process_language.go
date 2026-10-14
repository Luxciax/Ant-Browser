package backend

import "strings"

func browserProcessEnv(base []string, goos string, args []string) []string {
	out := append([]string(nil), base...)
	if goos != "linux" {
		return out
	}
	locale := browserArgValue(args, "--lang")
	if locale == "" {
		locale = browserLangFromAcceptLanguage(browserArgValue(args, "--accept-lang"))
	}
	if locale == "" {
		return out
	}
	language := strings.ReplaceAll(locale, "-", "_")
	if baseLang, _, ok := strings.Cut(language, "_"); ok {
		language += ":" + baseLang
	}
	filtered := out[:0]
	for _, v := range out {
		if !strings.HasPrefix(v, "LANGUAGE=") {
			filtered = append(filtered, v)
		}
	}
	return append(filtered, "LANGUAGE="+language)
}
