package backend

import (
	"ant-chrome/backend/internal/logger"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"time"
)

// Configure before command construction; detached inheritance is explicitly unknown.
func (a *App) configureKernelDiagnosticLog(plan *browserStartPlan) {
	for _, arg := range plan.args {
		if strings.HasPrefix(arg, "--enable-logging") {
			return
		}
	}
	plan.args = append(plan.args, "--enable-logging=stderr")
}

func (a *App) prepareKernelDiagnosticLog(plan *browserStartPlan, monitor *browserProcessMonitor) {
	logger.New("Diagnostics").Info("内核日志采集覆盖", logger.F("event", "diagnostics.kernel_coverage"), logger.F("critical", true), logger.F("profile_id", plan.profile.ProfileId), logger.F("run_id", plan.profile.LastStartAt), logger.F("stderr", "managed launcher pipe; detached process inheritance not guaranteed"), logger.F("file_logging", "Ant adds no raw file; managed stream is redacted before persistence; user-provided logging flags are preserved"), logger.F("kernel_binary", monitor.kernelBinary))
}

func kernelIdentity(path string) map[string]interface{} {
	out := map[string]interface{}{"path": path}
	f, err := os.Open(path)
	if err != nil {
		out["state"] = "unavailable"
		out["error"] = err.Error()
		return out
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		out["state"] = "unavailable"
		out["error"] = err.Error()
		return out
	}
	out["size_bytes"] = info.Size()
	out["modified_at"] = info.ModTime().Format(time.RFC3339Nano)
	if info.Size() > 16*1024*1024 {
		out["hash_status"] = "omitted: executable exceeds 16MB identity budget"
		return out
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		out["hash_status"] = err.Error()
	} else {
		out["sha256"] = hex.EncodeToString(h.Sum(nil))
		out["state"] = "captured"
	}
	return out
}
