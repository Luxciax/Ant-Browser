package backend

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func syncConfiguredBrowserWindowSize(port int, args []string) error {
	return syncConfiguredBrowserWindowSizeWithCall(port, args, cdpBrowserCallResult)
}

func syncConfiguredBrowserWindowSizeWithCall(port int, args []string, call func(int, string, map[string]any) (map[string]any, error)) error {
	parts := strings.Split(browserArgValue(args, "--window-size"), ",")
	if len(parts) != 2 {
		return nil
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || w < 100 || h < 100 || w > 16384 || h > 16384 {
		return fmt.Errorf("invalid configured window size")
	}
	result, err := call(port, "Target.getTargets", nil)
	if err != nil {
		return err
	}
	infos, _ := result["targetInfos"].([]any)
	seen := map[float64]bool{}
	for _, raw := range infos {
		target, ok := raw.(map[string]any)
		if !ok || target["type"] != "page" {
			continue
		}
		params := map[string]any{"targetId": target["targetId"]}
		result, err := call(port, "Browser.getWindowForTarget", params)
		if err != nil {
			return err
		}
		window, ok := result["windowId"].(float64)
		if !ok || seen[window] {
			continue
		}
		seen[window] = true
		bounds, _ := result["bounds"].(map[string]any)
		// Preserve an explicitly maximized/fullscreen/minimized restored window.
		// --window-size describes normal bounds, not the user's window state.
		if state, _ := bounds["windowState"].(string); state != "" && state != "normal" {
			continue
		}
		if bounds["width"] == float64(w) && bounds["height"] == float64(h) {
			continue
		}
		if _, err := call(port, "Browser.setWindowBounds", map[string]any{"windowId": window, "bounds": map[string]any{"width": w, "height": h}}); err != nil {
			return err
		}
		matched := false
		for attempt := 0; attempt < 3; attempt++ {
			result, err = call(port, "Browser.getWindowForTarget", params)
			if err != nil {
				return err
			}
			bounds, _ = result["bounds"].(map[string]any)
			actualW, _ := bounds["width"].(float64)
			actualH, _ := bounds["height"].(float64)
			if actualW >= float64(w-4) && actualW <= float64(w+4) && actualH >= float64(h-4) && actualH <= float64(h+4) {
				matched = true
				break
			}
			if attempt < 2 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if !matched {
			return fmt.Errorf("窗口尺寸校正未生效，期望 %d,%d", w, h)
		}
	}
	return nil
}
