package proxy

import (
	"ant-chrome/backend/internal/apppath"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type MihomoNodeBridge struct {
	NodeKey        string
	Port           int
	ControllerPort int
	Cmd            *exec.Cmd
	Pid            int
	ConfigPath     string
	Running        bool
	RefCount       int
	LastUsedAt     time.Time
	ExitDone       chan struct{}
	ExitErr        error
	Stopping       bool
	Restarting     bool
	RestartCount   int
}

func (m *ClashManager) EnsureNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	return m.EnsureNodeBridgeContext(context.Background(), proxyConfig, proxies, proxyId)

}

func (m *ClashManager) EnsureNodeBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, error) {
	proxyURL, _, err := m.ensureNodeBridgeContext(ctx, proxyConfig, proxies, proxyId, false)
	return proxyURL, err
}

func (m *ClashManager) AcquireNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string) (string, string, error) {
	return m.ensureNodeBridgeContext(context.Background(), proxyConfig, proxies, proxyId, true)
}

func (m *ClashManager) ReleaseNodeBridge(key string) {
	key = strings.TrimSpace(key)
	if key == "" || m == nil {
		return
	}

	var bridgeToStop *MihomoNodeBridge
	m.mu.Lock()
	bridge := m.NodeBridges[key]
	if bridge != nil {
		if bridge.RefCount > 0 {
			bridge.RefCount--
		}
		bridge.LastUsedAt = time.Now()
		if bridge.RefCount <= 0 {
			bridge.Stopping = true
			bridge.Running = false
			delete(m.NodeBridges, key)
			bridgeToStop = bridge
		}
	}
	m.mu.Unlock()

	if bridgeToStop != nil && bridgeToStop.Cmd != nil && bridgeToStop.Cmd.Process != nil {
		_ = bridgeToStop.Cmd.Process.Kill()
	}
}

func (m *ClashManager) ensureNodeBridge(proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	return m.ensureNodeBridgeContext(context.Background(), proxyConfig, proxies, proxyId, pin)
}

func (m *ClashManager) ensureNodeBridgeContext(ctx context.Context, proxyConfig string, proxies []config.BrowserProxy, proxyId string, pin bool) (string, string, error) {
	log := logger.New("Mihomo")
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	src := strings.TrimSpace(resolveProxyConfig(proxyConfig, proxies, proxyId))
	if src == "" {
		return "", "", fmt.Errorf("未找到代理节点")
	}
	if strings.EqualFold(src, "direct://") {
		return "direct://", "", nil
	}

	binaryPath, err := m.resolveMihomoBinary()
	if err != nil {
		return "", "", err
	}
	var (
		baseNodes      []interface{}
		targetName     string
		preferredPort  int
	)
	if IsChainProxy(src) {
		chainCfg, err := ParseChainProxyConfig(src)
		if err != nil {
			return "", "", err
		}
		baseNodes, targetName, err = buildMihomoReferencedChainNodes(chainCfg, proxies, proxyId)
		if err != nil {
			return "", "", err
		}
		preferredPort = chainCfg.LocalPort
	} else {
		node, err := buildMihomoNode(src)
		if err != nil {
			return "", "", err
		}
		baseNodes = []interface{}{node}
		targetName = strings.TrimSpace(getMapString(node, "name"))
	}

	baseKeySource := chainProxyRuntimeKeySource(src, proxies, proxyId) + "\x00mihomo"
	decisionKey := computeNodeKey(baseKeySource + "\x00egress-auto")
	var lastErr error

	for _, mode := range egressCandidateModes(decisionKey) {
		interfaceName, usable := egressInterfaceForMode(mode)
		if !usable {
			continue
		}
		nodes := cloneInterfaceSlice(baseNodes)
		key := computeNodeKey(baseKeySource + "\x00egress:" + string(mode) + ":" + interfaceName)

		unlock := m.lockLaunchForKey(key)
		if proxyURL, reused, reuseErr := m.tryReuseMihomoNodeBridgeContext(ctx, key, pin); reuseErr != nil {
			unlock()
			return "", "", reuseErr
		} else if reused {
			unlock()
			rememberEgressDecision(decisionKey, mode, interfaceName)
			log.Info("复用 mihomo 桥接",
				logger.F("engine", "mihomo"),
				logger.F("egress", string(mode)),
				logger.F("interface", interfaceName),
				logger.F("key", key[:8]),
				logger.F("proxy_url", proxyURL),
			)
			return proxyURL, key, nil
		}
		if err := ctx.Err(); err != nil {
			unlock()
			return "", "", err
		}

		port := preferredPort
		if port <= 0 {
			port, err = nextAvailablePort()
			if err != nil {
				unlock()
				lastErr = err
				continue
			}
		}
		controllerPort, portErr := nextAvailablePort()
		if portErr != nil {
			unlock()
			lastErr = portErr
			continue
		}
		cfgPath, cfgErr := m.buildMihomoNodesConfigWithInterface(key, nodes, targetName, port, controllerPort, interfaceName)
		if cfgErr != nil {
			unlock()
			lastErr = cfgErr
			continue
		}

		cmd := exec.Command(binaryPath, "-f", cfgPath, "-d", filepath.Dir(cfgPath))
		hideWindow(cmd)
		cmd.Dir = filepath.Dir(cfgPath)
		stderrPath := filepath.Join(filepath.Dir(cfgPath), "mihomo-stderr.log")
		stderrFile, captureErr := logger.NewProcessLog(stderrPath)
		if captureErr != nil {
			unlock()
			lastErr = fmt.Errorf("mihomo output capture: %w", captureErr)
			continue
		}
		if stderrFile != nil {
			cmd.Stderr = stderrFile
			cmd.Stdout = stderrFile
		}
		if startErr := cmd.Start(); startErr != nil {
			if stderrFile != nil {
				_ = stderrFile.Close()
			}
			unlock()
			lastErr = fmt.Errorf("mihomo 启动失败: %w", startErr)
			continue
		}

		bridge := &MihomoNodeBridge{
			NodeKey: key, Port: port, ControllerPort: controllerPort, Cmd: cmd,
			Pid: cmd.Process.Pid, ConfigPath: cfgPath, Running: true,
			LastUsedAt: time.Now(), ExitDone: make(chan struct{}),
		}
		if pin {
			bridge.RefCount = 1
		}

		readyErr := waitTCPPortReadyContext(ctx, "127.0.0.1", port, 10*time.Second)
		if readyErr == nil {
			readyErr = waitTCPPortReadyContext(ctx, "127.0.0.1", controllerPort, 10*time.Second)
		}
		if stderrFile != nil {
			_ = stderrFile.Close()
		}
		if readyErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			unlock()
			lastErr = fmt.Errorf("mihomo 本地端口未就绪: %w", readyErr)
			if preferredPort > 0 {
				time.Sleep(150 * time.Millisecond)
			}
			continue
		}
		if err := ctx.Err(); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			unlock()
			return "", "", err
		}

		proxyURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		if probeErr := probeLocalProxy(ctx, proxyURL); probeErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			unlock()
			lastErr = fmt.Errorf("%s 出口探测失败: %w", mode, probeErr)
			log.Warn("mihomo 出口候选不可用",
				logger.F("egress", string(mode)),
				logger.F("interface", interfaceName),
				logger.F("proxy_id", proxyId),
				logger.F("error", lastErr),
			)
			if preferredPort > 0 {
				time.Sleep(150 * time.Millisecond)
			}
			continue
		}

		m.registerMihomoNodeBridge(key, bridge)
		m.watchMihomoNodeBridge(bridge)
		unlock()
		rememberEgressDecision(decisionKey, mode, interfaceName)
		log.Info("mihomo 自动出口选择成功",
			logger.F("engine", "mihomo"),
			logger.F("egress", string(mode)),
			logger.F("interface", interfaceName),
			logger.F("proxy_id", proxyId),
		)
		return proxyURL, key, nil
	}

	forgetEgressDecision(decisionKey)
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的物理或系统出口")
	}
	return "", "", fmt.Errorf("mihomo 自动出口选择失败: %w", lastErr)
}

func (m *ClashManager) tryReuseMihomoNodeBridge(key string, pin bool) (string, bool) {
	proxyURL, reused, _ := m.tryReuseMihomoNodeBridgeContext(context.Background(), key, pin)
	return proxyURL, reused
}

func (m *ClashManager) tryReuseMihomoNodeBridgeContext(ctx context.Context, key string, pin bool) (string, bool, error) {
	if m == nil {
		return "", false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	m.mu.Lock()
	if m.NodeBridges == nil {
		m.NodeBridges = map[string]*MihomoNodeBridge{}
	}
	bridge := m.NodeBridges[key]
	if bridge != nil && bridge.RefCount > 0 && !bridge.Stopping {
		if pin {
			bridge.RefCount++
		}
		bridge.LastUsedAt = time.Now()
		m.mu.Unlock()
		return fmt.Sprintf("http://127.0.0.1:%d", bridge.Port), true, nil
	}
	if bridge == nil || !bridge.Running || bridge.Cmd == nil || bridge.Cmd.Process == nil || bridge.Cmd.ProcessState != nil {
		delete(m.NodeBridges, key)
		m.mu.Unlock()
		return "", false, nil
	}
	m.mu.Unlock()
	if err := waitTCPPortReadyContext(ctx, "127.0.0.1", bridge.Port, 800*time.Millisecond); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", false, ctxErr
		}
		_ = bridge.Cmd.Process.Kill()
		m.mu.Lock()
		if m.NodeBridges[key] == bridge {
			bridge.Running = false
			delete(m.NodeBridges, key)
		}
		m.mu.Unlock()
		return "", false, nil
	}
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	m.mu.Lock()
	if m.NodeBridges[key] != bridge || !bridge.Running {
		m.mu.Unlock()
		return "", false, nil
	}
	if pin {
		bridge.RefCount++
	}
	bridge.LastUsedAt = time.Now()
	m.mu.Unlock()
	return fmt.Sprintf("http://127.0.0.1:%d", bridge.Port), true, nil
}

func (m *ClashManager) discardMihomoNodeBridge(key string, bridge *MihomoNodeBridge) {
	if bridge == nil {
		return
	}
	m.mu.Lock()
	bridge.Stopping = true
	bridge.Running = false
	if current, ok := m.NodeBridges[key]; ok && current == bridge {
		delete(m.NodeBridges, key)
	}
	m.mu.Unlock()
	if bridge.Cmd != nil && bridge.Cmd.Process != nil {
		_ = bridge.Cmd.Process.Kill()
	}
}

func (m *ClashManager) TestNodeDelay(proxyId string, proxies []config.BrowserProxy, cfg *SpeedTestConfig) TestResult {
	src := strings.TrimSpace(resolveProxyConfig("", proxies, proxyId))
	if src == "" {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "代理配置为空"}
	}
	if strings.EqualFold(src, "direct://") {
		return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: 0, Engine: "direct"}
	}
	if m == nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 管理器未初始化"}
	}
	if _, err := m.EnsureNodeBridge(src, proxies, proxyId); err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: err.Error()}
	}
	baseKeySource := chainProxyRuntimeKeySource(src, proxies, proxyId) + "\x00mihomo"
	key, mode, _, ok := selectedEgressRuntimeKey(baseKeySource)
	if !ok {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: "mihomo", Error: "mihomo 自动出口状态不可用"}
	}
	m.mu.Lock()
	bridge := m.NodeBridges[key]
	m.mu.Unlock()
	engine := "mihomo"
	if label := egressModeLabel(mode); label != "" {
		engine += " · " + label
	}
	if bridge == nil || !bridge.Running || bridge.ControllerPort <= 0 {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "mihomo 控制端口未就绪"}
	}
	timeout := 10 * time.Second
	testURL := DefaultSpeedTestURL
	if cfg != nil {
		if cfg.Timeout > 0 {
			timeout = cfg.Timeout
		}
		if urls := normalizeSpeedTestURLs(cfg.URLs); len(urls) > 0 {
			testURL = urls[0]
		}
	}
	apiURL := fmt.Sprintf(
		"http://127.0.0.1:%d/proxies/%s/delay?timeout=%d&url=%s",
		bridge.ControllerPort,
		url.PathEscape("proxy-out"),
		int(timeout.Milliseconds()),
		url.QueryEscape(testURL),
	)
	client := &http.Client{Timeout: timeout + time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "mihomo 延迟测试失败: " + err.Error()}
	}
	defer resp.Body.Close()
	var payload struct {
		Delay int64  `json:"delay"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "mihomo 延迟结果解析失败: " + err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if payload.Error != "" {
			return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: payload.Error}
		}
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: fmt.Sprintf("mihomo 延迟测试失败: HTTP %d", resp.StatusCode)}
	}
	if payload.Delay <= 0 {
		if payload.Error != "" {
			return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: payload.Error}
		}
		return TestResult{ProxyId: proxyId, Ok: false, Engine: engine, Error: "mihomo 延迟测试无结果"}
	}
	return TestResult{ProxyId: proxyId, Ok: true, LatencyMs: payload.Delay, Engine: engine}
}

func (m *ClashManager) registerMihomoNodeBridge(key string, bridge *MihomoNodeBridge) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.NodeBridges == nil {
		m.NodeBridges = map[string]*MihomoNodeBridge{}
	}
	if old := m.NodeBridges[key]; old != nil && old.Cmd != nil && old.Cmd.Process != nil {
		old.Stopping = true
		old.Running = false
		_ = old.Cmd.Process.Kill()
	}
	m.NodeBridges[key] = bridge
}

func (m *ClashManager) watchMihomoNodeBridge(bridge *MihomoNodeBridge) {
	if m == nil || bridge == nil || bridge.Cmd == nil {
		return
	}
	go func() {
		err := bridge.Cmd.Wait()
		m.mu.Lock()
		bridge.Running = false
		bridge.ExitErr = err
		current := m.NodeBridges[bridge.NodeKey] == bridge
		recoverPinned := current && !bridge.Stopping && bridge.RefCount > 0 && bridge.RestartCount < 3
		if recoverPinned {
			bridge.Restarting = true
		} else if current {
			delete(m.NodeBridges, bridge.NodeKey)
		}
		stopping := bridge.Stopping
		m.mu.Unlock()
		close(bridge.ExitDone)
		if !current || stopping {
			return
		}
		log := logger.New("Mihomo")
		if m.OnBridgeEvidence != nil {
			m.OnBridgeEvidence(bridge.NodeKey, "proxy.bridge_exited", map[string]interface{}{"process": bridgeExitEvidence(bridge.Cmd), "stderr_tail": bridgeStderrTail(bridge.Cmd, "mihomo-stderr.log"), "port": bridge.Port, "recovery_requested": recoverPinned})
		}
		log.Warn("mihomo 桥接进程退出", logger.F("key", bridge.NodeKey), logger.F("port", bridge.Port), logger.F("exit_error", err), logger.F("process_evidence", bridgeExitEvidence(bridge.Cmd)), logger.F("stderr_tail", bridgeStderrTail(bridge.Cmd, "mihomo-stderr.log")), logger.F("restart_count", bridge.RestartCount))
		if recoverPinned {
			if recoveryErr := m.restartMihomoNodeBridge(bridge); recoveryErr == nil {
				if m.OnBridgeEvidence != nil {
					m.OnBridgeEvidence(bridge.NodeKey, "proxy.bridge_recovered", map[string]interface{}{"port": bridge.Port})
				}
				return
			} else {
				err = recoveryErr
			}
		}
		m.mu.Lock()
		if m.NodeBridges[bridge.NodeKey] == bridge {
			delete(m.NodeBridges, bridge.NodeKey)
		}
		stopping = bridge.Stopping
		m.mu.Unlock()
		if !stopping {
			log.Error("mihomo 桥接恢复失败", logger.F("key", bridge.NodeKey), logger.F("port", bridge.Port), logger.F("error", err))
			if m.OnBridgeDied != nil {
				m.OnBridgeDied(bridge.NodeKey, fmt.Errorf("mihomo 桥接退出，恢复失败或达到恢复上限：%v", err))
			}
		}
	}()
}

func (m *ClashManager) lockLaunchForKey(key string) func() {
	if m == nil {
		return func() {}
	}
	m.mu.Lock()
	if m.launchLocks == nil {
		m.launchLocks = make(map[string]*bridgeLaunchLock)
	}
	lock := m.launchLocks[key]
	if lock == nil {
		lock = &bridgeLaunchLock{}
		m.launchLocks[key] = lock
	}
	lock.refs++
	m.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		m.mu.Lock()
		lock.refs--
		if lock.refs <= 0 && m.launchLocks[key] == lock {
			delete(m.launchLocks, key)
		}
		m.mu.Unlock()
	}
}

func (m *ClashManager) buildMihomoNodeConfig(key string, node map[string]interface{}, port int, controllerPort int) (string, error) {
	return m.buildMihomoNodesConfig(key, []interface{}{node}, strings.TrimSpace(getMapString(node, "name")), port, controllerPort)
}

func (m *ClashManager) buildMihomoNodesConfig(key string, nodes []interface{}, targetName string, port int, controllerPort int) (string, error) {
	return m.buildMihomoNodesConfigWithInterface(key, nodes, targetName, port, controllerPort, "")
}

func (m *ClashManager) buildMihomoNodesConfigWithInterface(key string, nodes []interface{}, targetName string, port int, controllerPort int, interfaceName string) (string, error) {
	baseDir := m.resolveMihomoWorkdir(key)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("mihomo 节点配置为空")
	}
	name := strings.TrimSpace(targetName)
	if name == "" {
		if node, ok := nodes[0].(map[string]interface{}); ok {
			name = strings.TrimSpace(getMapString(node, "name"))
			if name == "" {
				name = "node"
				node["name"] = name
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("mihomo 目标节点名称为空")
	}
	payload := map[string]interface{}{
		"mixed-port":          port,
		"external-controller": fmt.Sprintf("127.0.0.1:%d", controllerPort),
		"allow-lan":           false,
		"mode":                "rule",
		"log-level":           "warning",
		"ipv6":                true,
		"unified-delay":       true,
		"tcp-concurrent":      false,
		"proxies":             nodes,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name":    "proxy-out",
				"type":    "select",
				"proxies": []interface{}{name},
			},
		},
		"rules": []interface{}{"MATCH,proxy-out"},
	}
	if interfaceName = strings.TrimSpace(interfaceName); interfaceName != "" {
		payload["interface-name"] = interfaceName
	}
	cfgPath := filepath.Join(baseDir, "mihomo-config.yaml")
	data, err := yaml.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func buildMihomoNode(src string) (map[string]interface{}, error) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(src), &payload); err == nil {
		if node := pickClashNode(payload); node != nil {
			return cloneStringInterfaceMap(node), nil
		}
	}
	mapping, err := proxyConfigToMapping(src)
	if err != nil {
		return nil, fmt.Errorf("mihomo 节点解析失败: %w", err)
	}
	out := map[string]interface{}{}
	for key, value := range mapping {
		out[key] = value
	}
	return out, nil
}

func (m *ClashManager) resolveMihomoBinary() (string, error) {
	if m == nil || m.Config == nil {
		return "", fmt.Errorf("mihomo 管理器未初始化")
	}
	candidates := []string{}
	if configured := strings.TrimSpace(m.Config.Browser.ClashBinaryPath); configured != "" {
		candidates = append(candidates, resolveEnvPath(configured, m.AppRoot))
	}
	for _, name := range []string{"mihomo.exe", "mihomo", "clash-meta.exe", "clash-meta", "clash.exe", "clash"} {
		if path, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, path)
		}
	}
	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			candidates = append(candidates,
				filepath.Join(appData, "mihomo-party", "mihomo.exe"),
				filepath.Join(appData, "mihomo-party", "core", "mihomo.exe"),
				filepath.Join(appData, "mihomo-party", "core", "mihomo-windows-amd64.exe"),
			)
		}
	}
	if m.AppRoot != "" {
		candidates = append(candidates,
			filepath.Join(m.AppRoot, "bin", "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", "mihomo"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo", "mihomo"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo.exe"),
			filepath.Join(m.AppRoot, "bin", runtime.GOOS+"-"+runtime.GOARCH, "mihomo"),
		)
	}
	for _, candidate := range candidates {
		candidate = fsutil.NormalizePathInput(candidate)
		if candidate == "" {
			continue
		}
		if !filepath.IsAbs(candidate) && m.AppRoot != "" {
			candidate = apppath.Resolve(m.AppRoot, candidate)
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if err := fsutil.EnsureExecutable(candidate); err != nil {
				return "", fmt.Errorf("mihomo 文件不可执行: %s: %w", candidate, err)
			}
			return candidate, nil
		}
	}
	return "", fmt.Errorf("mihomo/clash 可执行文件未找到，请配置 browser.clash_binary_path")
}

func (m *ClashManager) resolveMihomoWorkdir(key string) string {
	root := "data"
	if m != nil && m.Config != nil {
		root = strings.TrimSpace(m.Config.Browser.UserDataRoot)
		if root == "" {
			root = "data"
		}
	}
	if !filepath.IsAbs(root) && m != nil {
		root = apppath.Resolve(m.AppRoot, root)
	}
	return filepath.Join(root, "_mihomo", key)
}

func waitTCPPortReady(host string, port int, timeout time.Duration) error {
	return waitTCPPortReadyContext(context.Background(), host, port, timeout)
}

func waitTCPPortReadyContext(ctx context.Context, host string, port int, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort(host, strconv.Itoa(port))
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		probeTimeout := 200 * time.Millisecond
		if remaining := time.Until(deadline); remaining < probeTimeout {
			probeTimeout = remaining
		}
		dialer := &net.Dialer{Timeout: probeTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if time.Now().After(deadline) {
			return err
		}
		pause := 100 * time.Millisecond
		if remaining := time.Until(deadline); remaining < pause {
			pause = remaining
		}
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
