package browser

import (
	"os/exec"
	"strconv"
	"syscall"
)

func hideCoreProbeWindow(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func stopCoreProbeProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Only the process tree created by this isolated probe is targeted.
	kill := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	hideCoreProbeWindow(kill)
	_ = kill.Run()
}
