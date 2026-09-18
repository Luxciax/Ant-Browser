package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const browserStartReadyTimeout = 10 * time.Second
const browserStartStableWindow = 1200 * time.Millisecond

var errBrowserDebugPortPending = errors.New("browser debug port pending")

func waitBrowserDebugPortReady(initialDebugPort int, userDataDir string, timeout time.Duration, monitor *browserProcessMonitor) (int, error) {
	return waitBrowserDebugPortReadyContext(context.Background(), initialDebugPort, userDataDir, timeout, monitor)
}

func waitBrowserDebugPortReadyContext(ctx context.Context, initialDebugPort int, userDataDir string, timeout time.Duration, monitor *browserProcessMonitor) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	allowDetachedGrace := initialDebugPort > 0
	var lastErr error
	var exitResult browserProcessExitResult
	exitObserved := false

	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor)
		if resolveErr == nil {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return debugPort, nil
			} else {
				lastErr = err
			}
		} else if !errors.Is(resolveErr, errBrowserDebugPortPending) {
			lastErr = resolveErr
		}
		if monitor != nil && monitor.HasExited() {
			if !exitObserved {
				exitResult = monitor.Result()
				exitObserved = true
				if !allowDetachedGrace {
					return 0, newBrowserStartupExitError(exitResult)
				}
				exitDeadline := time.Now().Add(browserLauncherDetachGraceWindow)
				if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(exitDeadline) {
					exitDeadline = ctxDeadline
				}
				if exitDeadline.After(deadline) {
					deadline = exitDeadline
				}
			}
		}
		if !sleepWithContext(ctx, 150*time.Millisecond) {
			return 0, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !exitObserved && monitor != nil && monitor.HasExited() {
		exitResult = monitor.Result()
		exitObserved = true
		if !allowDetachedGrace {
			return 0, newBrowserStartupExitError(exitResult)
		}
		postExitDeadline := time.Now().Add(browserLauncherDetachGraceWindow)
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(postExitDeadline) {
			postExitDeadline = ctxDeadline
		}
		for time.Now().Before(postExitDeadline) {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
				if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
					return debugPort, nil
				}
			}
			if !sleepWithContext(ctx, 150*time.Millisecond) {
				return 0, ctx.Err()
			}
		}
	}
	if exitObserved {
		if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
			if err := probeBrowserDebugPort(debugPort, browserDebugProbeTimeout); err == nil {
				return debugPort, nil
			}
		}
		return 0, newBrowserStartupExitError(exitResult)
	}
	if lastErr != nil {
		if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
			return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪：%w", timeout.Round(time.Second), debugPort, lastErr)
		}
		return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，尚未获取调试端口：%w", timeout.Round(time.Second), lastErr)
	}

	if debugPort, resolveErr := resolveBrowserDebugPort(initialDebugPort, userDataDir, monitor); resolveErr == nil {
		return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，调试端口 %d 未就绪", timeout.Round(time.Second), debugPort)
	}

	return 0, fmt.Errorf("浏览器进程未在 %s 内完成启动，尚未获取调试端口", timeout.Round(time.Second))
}

func waitBrowserDebugPortStable(initialDebugPort int, userDataDir string, timeout time.Duration, stableFor time.Duration, monitor *browserProcessMonitor) (int, error) {
	return waitBrowserDebugPortStableContext(context.Background(), initialDebugPort, userDataDir, timeout, stableFor, monitor)
}

func waitBrowserDebugPortStableContext(ctx context.Context, initialDebugPort int, userDataDir string, timeout time.Duration, stableFor time.Duration, monitor *browserProcessMonitor) (int, error) {
	debugPort, err := waitBrowserDebugPortReadyContext(ctx, initialDebugPort, userDataDir, timeout, monitor)
	if err != nil {
		return 0, err
	}
	if stableFor <= 0 {
		return debugPort, nil
	}
	allowDetachedGrace := initialDebugPort > 0
	return stabilizeBrowserDebugPortContext(ctx, debugPort, stableFor, allowDetachedGrace, monitor, probeBrowserDebugPort)
}

func stabilizeBrowserDebugPort(debugPort int, stableFor time.Duration, allowDetachedGrace bool, monitor *browserProcessMonitor, probe func(int, time.Duration) error) (int, error) {
	return stabilizeBrowserDebugPortContext(context.Background(), debugPort, stableFor, allowDetachedGrace, monitor, probe)
}

func stabilizeBrowserDebugPortContext(ctx context.Context, debugPort int, stableFor time.Duration, allowDetachedGrace bool, monitor *browserProcessMonitor, probe func(int, time.Duration) error) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(stableFor)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	consecutiveFailures := 0
	const maxStableProbeFailures = 2
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		if monitor != nil && monitor.HasExited() {
			if !allowDetachedGrace {
				return 0, newBrowserStartupExitError(monitor.Result())
			}
		}
		probeTimeout := browserDebugProbeTimeout
		if remaining < probeTimeout {
			probeTimeout = remaining
		}
		if err := probe(debugPort, probeTimeout); err != nil {
			consecutiveFailures++
			if consecutiveFailures > maxStableProbeFailures {
				return 0, fmt.Errorf("浏览器调试端口 %d 稳定窗口内连续失败：%w", debugPort, err)
			}
		} else {
			consecutiveFailures = 0
		}

		remaining = time.Until(deadline)
		if remaining <= 0 {
			break
		}
		sleepFor := 150 * time.Millisecond
		if remaining < sleepFor {
			sleepFor = remaining
		}
		if !sleepWithContext(ctx, sleepFor) {
			return 0, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	if monitor != nil && monitor.HasExited() && !allowDetachedGrace {
		return 0, newBrowserStartupExitError(monitor.Result())
	}
	if err := probe(debugPort, browserDebugProbeTimeout); err != nil {
		return 0, fmt.Errorf("浏览器调试端口 %d 稳定窗口最终探测失败：%w", debugPort, err)
	}
	if monitor != nil && monitor.HasExited() && !allowDetachedGrace {
		return 0, newBrowserStartupExitError(monitor.Result())
	}
	return debugPort, nil
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return true
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func resolveBrowserDebugPort(initialDebugPort int, userDataDir string, monitor *browserProcessMonitor) (int, error) {
	if initialDebugPort > 0 {
		return initialDebugPort, nil
	}
	if monitor != nil {
		if debugPort, ok := monitor.DebugPort(); ok {
			return debugPort, nil
		}
	}
	if debugPort, err := readBrowserDebugPortFile(userDataDir); err == nil {
		if monitor != nil {
			monitor.SetDebugPort(debugPort)
		}
		return debugPort, nil
	} else if !errors.Is(err, errBrowserDebugPortPending) {
		return 0, err
	}
	return 0, errBrowserDebugPortPending
}

func readBrowserDebugPortFile(userDataDir string) (int, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return 0, errBrowserDebugPortPending
	}

	data, err := os.ReadFile(filepath.Join(userDataDir, "DevToolsActivePort"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, errBrowserDebugPortPending
		}
		return 0, fmt.Errorf("读取 DevToolsActivePort 失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return 0, errBrowserDebugPortPending
	}

	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("DevToolsActivePort 内容无效: %q", lines[0])
	}
	return port, nil
}
