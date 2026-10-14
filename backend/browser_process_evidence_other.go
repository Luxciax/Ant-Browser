//go:build !windows

package backend

import "syscall"

type processEvidenceHandle struct {
	pid     int
	created int64
	openErr error
}

func openProcessEvidence(pid int) *processEvidenceHandle { return &processEvidenceHandle{pid: pid} }
func (p *processEvidenceHandle) snapshot() map[string]interface{} {
	out := map[string]interface{}{"pid": p.pid, "state": "unknown", "exit_code": nil, "source": "process_probe", "limitation": "retained Windows process handle unavailable on this platform"}
	err := syscall.Kill(p.pid, 0)
	if err == nil {
		out["state"] = "alive"
	} else if err == syscall.ESRCH {
		out["state"] = "exited"
	} else {
		out["error"] = err.Error()
	}
	return out
}
func (p *processEvidenceHandle) close() {}
func systemResourceEvidence() map[string]interface{} {
	return map[string]interface{}{"state": "unavailable", "reason": "native resource sampler currently supports Windows"}
}

func diagnosticDiskEvidence(path string) map[string]interface{} {
	return map[string]interface{}{"state": "unavailable", "reason": "native disk sampler currently supports Windows"}
}
