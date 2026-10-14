//go:build !windows

package backend

import "time"

func collectCrashEvidence(at time.Time, snapshot map[string]interface{}, core string) map[string]interface{} {
	return map[string]interface{}{"state": "unavailable", "reason": "Windows event evidence is not available on this platform", "binary_dump_exported": false}
}
