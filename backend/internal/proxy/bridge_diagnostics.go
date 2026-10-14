package proxy

import (
	"ant-chrome/backend/internal/logger"
	"os/exec"
	"path/filepath"
)

func bridgeStderrTail(cmd *exec.Cmd, name string) string {
	if cmd == nil || cmd.Dir == "" {
		return ""
	}
	if stream, ok := cmd.Stderr.(*logger.ProcessLog); ok {
		_ = stream.Close()
		return readLogTail(stream.Path, 64*1024)
	}
	return logger.RedactDiagnosticText(readLogTail(filepath.Join(cmd.Dir, name), 64*1024))
}

func bridgeExitEvidence(cmd *exec.Cmd) map[string]interface{} {
	out := map[string]interface{}{"exit_code": nil}
	if cmd == nil {
		return out
	}
	if cmd.Process != nil {
		out["pid"] = cmd.Process.Pid
	}
	if cmd.ProcessState != nil {
		out["exit_code"] = cmd.ProcessState.ExitCode()
	}
	if stream, ok := cmd.Stderr.(*logger.ProcessLog); ok {
		out["output_path"] = stream.Path
		out["bridge_run_id"] = filepath.Base(stream.Path)
	}
	return out
}
