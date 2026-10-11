package proxy

import (
	"os/exec"
	"path/filepath"
)

func bridgeStderrTail(cmd *exec.Cmd, name string) string {
	if cmd == nil || cmd.Dir == "" {
		return ""
	}
	return readLogTail(filepath.Join(cmd.Dir, name), 4096)
}
