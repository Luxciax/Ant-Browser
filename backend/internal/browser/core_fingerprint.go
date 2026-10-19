package browser

import (
	"ant-chrome/backend/internal/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CoreFingerprintManifest struct {
	SchemaVersion      int                       `json:"schemaVersion"`
	Provider           string                    `json:"provider"`
	FingerprintBackend config.FingerprintBackend `json:"fingerprintBackend"`
	ChromiumVersion    string                    `json:"chromiumVersion,omitempty"`
	DetectedAt         string                    `json:"detectedAt"`
}

func coreManifestPath(corePath string) string {
	if info, err := os.Stat(corePath); err == nil && !info.IsDir() {
		corePath = filepath.Dir(corePath)
	}
	return filepath.Join(corePath, "ant-core.json")
}

func ReadCoreFingerprintManifest(corePath string) (CoreFingerprintManifest, bool) {
	var manifest CoreFingerprintManifest
	raw, err := os.ReadFile(coreManifestPath(corePath))
	if err != nil || json.Unmarshal(raw, &manifest) != nil || manifest.SchemaVersion != 1 || !manifest.FingerprintBackend.Valid() {
		return manifest, false
	}
	return manifest, true
}

func WriteCoreFingerprintManifest(corePath string, manifest CoreFingerprintManifest) error {
	manifest.SchemaVersion = 1
	if !manifest.FingerprintBackend.Valid() {
		return fmt.Errorf("invalid fingerprint backend")
	}
	if manifest.DetectedAt == "" {
		manifest.DetectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(coreManifestPath(corePath), append(raw, '\n'), 0600)
}

// Source attribution is limited to exact official GitHub release paths, never names.
func FingerprintBackendForDownload(source string) (config.FingerprintBackend, string) {
	parsed, err := url.Parse(source)
	if err != nil || !strings.EqualFold(parsed.Host, "github.com") || parsed.Scheme != "https" {
		return "", ""
	}
	path := strings.ToLower(parsed.Path)
	if strings.HasPrefix(path, "/clearcotelabs/clearcote-browser/releases/download/") {
		return config.FingerprintBackendClearcote, "clearcote"
	}
	if strings.HasPrefix(path, "/adryfish/fingerprint-chromium/releases/download/") {
		return config.FingerprintBackendFingerprintChromium, "fingerprint-chromium"
	}
	return "", ""
}

func (m *Manager) DetectCoreFingerprintBackend(corePath string) (config.FingerprintBackend, string) {
	absolute := m.ResolveRelativePath(corePath)
	if manifest, ok := ReadCoreFingerprintManifest(absolute); ok {
		return manifest.FingerprintBackend, manifest.Provider
	}
	executable, _, ok := FindCoreExecutable(absolute)
	if !ok {
		return config.FingerprintBackendGenericChromium, "unknown"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if ProbeClearcoteCompatibility(ctx, executable, m.ResolveRelativePath("data/core-probe")) == nil {
		return config.FingerprintBackendClearcote, "clearcote-compatible"
	}
	return config.FingerprintBackendGenericChromium, "unknown"
}

// Probe uses its own process/profile and an OS-assigned debugging port. No user profile is opened.
func ProbeClearcoteCompatibility(ctx context.Context, executable, probeRoot string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := os.MkdirAll(probeRoot, 0700); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(probeRoot, "probe-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	cmd := exec.Command(executable, "--headless=new", "--no-first-run", "--no-default-browser-check", "--user-data-dir="+directory, "--remote-debugging-address=127.0.0.1", "--remote-debugging-port=0", "--fingerprint-device-memory=4", "--fingerprint-max-touch-points=7", "about:blank")
	hideCoreProbeWindow(cmd)
	if err = cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		stopCoreProbeProcess(cmd)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}()
	var port int
	for port == 0 {
		raw, readErr := os.ReadFile(filepath.Join(directory, "DevToolsActivePort"))
		if readErr == nil {
			port, _ = strconv.Atoi(strings.SplitN(string(raw), "\n", 2)[0])
		}
		if port > 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			return fmt.Errorf("probe exited: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	var targets []struct {
		Type string `json:"type"`
		URL  string `json:"webSocketDebuggerUrl"`
	}
	for len(targets) == 0 {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", port), nil)
		response, requestErr := client.Do(req)
		if requestErr == nil {
			_ = json.NewDecoder(response.Body).Decode(&targets)
			response.Body.Close()
		}
		if len(targets) > 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	var socketURL string
	for _, target := range targets {
		if target.Type == "page" {
			socketURL = target.URL
			break
		}
	}
	if socketURL == "" {
		return fmt.Errorf("probe page unavailable")
	}
	socket, _, err := websocket.DefaultDialer.DialContext(ctx, socketURL, nil)
	if err != nil {
		return err
	}
	defer socket.Close()
	deadline, _ := ctx.Deadline()
	_ = socket.SetReadDeadline(deadline)
	_ = socket.SetWriteDeadline(deadline)
	if err = socket.WriteJSON(map[string]any{"id": 1, "method": "Runtime.evaluate", "params": map[string]any{"expression": "({deviceMemory:navigator.deviceMemory,maxTouchPoints:navigator.maxTouchPoints})", "returnByValue": true}}); err != nil {
		return err
	}
	for {
		var message struct {
			ID     int `json:"id"`
			Result struct {
				Result struct {
					Value struct {
						DeviceMemory   float64 `json:"deviceMemory"`
						MaxTouchPoints int     `json:"maxTouchPoints"`
					} `json:"value"`
				} `json:"result"`
			} `json:"result"`
		}
		if err = socket.ReadJSON(&message); err != nil {
			return err
		}
		if message.ID != 1 {
			continue
		}
		if message.Result.Result.Value.DeviceMemory != 4 || message.Result.Result.Value.MaxTouchPoints != 7 {
			return fmt.Errorf("native Clearcote overrides not detected")
		}
		_ = socket.WriteJSON(map[string]any{"id": 2, "method": "Browser.close"})
		return nil
	}
}
