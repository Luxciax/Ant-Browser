package backend

import "strings"

func diagnosticLaunchArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			out = append(out, arg)
		} else {
			out = append(out, "[startup target omitted]")
		}
	}
	return out
}
