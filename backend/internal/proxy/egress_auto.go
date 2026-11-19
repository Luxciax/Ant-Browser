package proxy

import (
	"ant-chrome/backend/internal/config"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type outboundEgressMode string

const (
	outboundEgressPhysical outboundEgressMode = "physical"
	outboundEgressSystem   outboundEgressMode = "system"
	egressDecisionTTL                         = 30 * time.Minute
	egressProbeTimeout                        = 4 * time.Second
)

type egressDecision struct {
	Mode      outboundEgressMode
	Interface string
	ExpiresAt time.Time
}

var egressDecisionCache sync.Map

func egressCandidateModes(cacheKey string) []outboundEgressMode {
	if cached, ok := loadEgressDecision(cacheKey); ok {
		if cached.Mode == outboundEgressPhysical {
			return []outboundEgressMode{outboundEgressPhysical, outboundEgressSystem}
		}
		return []outboundEgressMode{outboundEgressSystem, outboundEgressPhysical}
	}
	return []outboundEgressMode{outboundEgressPhysical, outboundEgressSystem}
}

func loadEgressDecision(cacheKey string) (egressDecision, bool) {
	value, ok := egressDecisionCache.Load(strings.TrimSpace(cacheKey))
	if !ok {
		return egressDecision{}, false
	}
	item, ok := value.(egressDecision)
	if !ok || time.Now().After(item.ExpiresAt) {
		egressDecisionCache.Delete(strings.TrimSpace(cacheKey))
		return egressDecision{}, false
	}
	if item.Mode == outboundEgressPhysical {
		current := detectPhysicalInterface()
		if current == "" || !strings.EqualFold(current, item.Interface) {
			egressDecisionCache.Delete(strings.TrimSpace(cacheKey))
			return egressDecision{}, false
		}
	}
	return item, true
}

func rememberEgressDecision(cacheKey string, mode outboundEgressMode, interfaceName string) {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return
	}
	egressDecisionCache.Store(cacheKey, egressDecision{
		Mode:      mode,
		Interface: strings.TrimSpace(interfaceName),
		ExpiresAt: time.Now().Add(egressDecisionTTL),
	})
}

func forgetEgressDecision(cacheKey string) {
	egressDecisionCache.Delete(strings.TrimSpace(cacheKey))
}

func egressInterfaceForMode(mode outboundEgressMode) (string, bool) {
	if mode != outboundEgressPhysical {
		return "", true
	}
	name := detectPhysicalInterface()
	return name, name != ""
}

type interfaceCandidate struct {
	Name  string
	Score int
}

func detectPhysicalInterface() string {
	if override := strings.TrimSpace(os.Getenv("ANT_PROXY_DIRECT_INTERFACE")); override != "" {
		if iface, err := net.InterfaceByName(override); err == nil && interfaceUsable(iface) {
			return iface.Name
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	candidates := make([]interfaceCandidate, 0, len(ifaces))
	for i := range ifaces {
		iface := &ifaces[i]
		if !interfaceUsable(iface) {
			continue
		}
		score := physicalInterfaceScore(iface.Name)
		if score <= -100 {
			continue
		}
		candidates = append(candidates, interfaceCandidate{Name: iface.Name, Score: score})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates[0].Name
}

func interfaceUsable(iface *net.Interface) bool {
	if iface == nil || iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var ip net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			ip = value.IP
		case *net.IPAddr:
			ip = value.IP
		}
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		if ip.To4() != nil {
			return true
		}
	}
	return false
}

func physicalInterfaceScore(name string) int {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return -1000
	}
	virtualMarkers := []string{
		"tun", "tap", "wintun", "v2ray", "xray", "sing-box", "singbox", "mihomo", "clash",
		"wireguard", "tailscale", "zerotier", "hamachi", "vpn", "docker", "container",
		"vmware", "virtualbox", "hyper-v", "loopback", "pseudo",
	}
	for _, marker := range virtualMarkers {
		if strings.Contains(lower, marker) {
			return -1000
		}
	}
	score := 10
	preferredMarkers := []string{"ethernet", "wi-fi", "wifi", "wlan", "以太网", "无线"}
	for _, marker := range preferredMarkers {
		if strings.Contains(lower, marker) {
			score += 100
		}
	}
	if strings.Contains(lower, "local area connection") {
		score += 40
	}
	return score
}

func cloneOutboundsForEgress(outbounds []interface{}) []interface{} {
	if len(outbounds) == 0 {
		return nil
	}
	cloned := make([]interface{}, len(outbounds))
	for i, item := range outbounds {
		cloned[i] = deepCloneEgressValue(item)
	}
	return cloned
}

func deepCloneEgressValue(value interface{}) interface{} {
	switch item := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(item))
		for key, nested := range item {
			cloned[key] = deepCloneEgressValue(nested)
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, len(item))
		for i, nested := range item {
			cloned[i] = deepCloneEgressValue(nested)
		}
		return cloned
	case []string:
		return append([]string(nil), item...)
	default:
		return value
	}
}

func selectedEgressRuntimeKey(baseKeySource string) (string, outboundEgressMode, string, bool) {
	decisionKey := computeNodeKey(baseKeySource + "\x00egress-auto")
	decision, ok := loadEgressDecision(decisionKey)
	if !ok {
		return "", "", "", false
	}
	interfaceName := strings.TrimSpace(decision.Interface)
	if decision.Mode == outboundEgressSystem {
		interfaceName = ""
	}
	key := computeNodeKey(baseKeySource + "\x00egress:" + string(decision.Mode) + ":" + interfaceName)
	return key, decision.Mode, interfaceName, true
}

func egressModeLabel(mode outboundEgressMode) string {
	if mode == outboundEgressPhysical {
		return "Direct"
	}
	if mode == outboundEgressSystem {
		return "System/TUN"
	}
	return ""
}

func egressBaseKeySourceForKernel(kernel string, src string, proxies []config.BrowserProxy, proxyId string) string {
	base := chainProxyRuntimeKeySource(normalizeNodeScheme(src), proxies, proxyId)
	switch NormalizePreferredKernel(kernel) {
	case ProxyKernelSingBox:
		return base
	case ProxyKernelMihomo:
		return base + "\x00mihomo"
	case ProxyKernelXray:
		dnsServers := ""
		if strings.TrimSpace(proxyId) != "" {
			for _, item := range proxies {
				if strings.EqualFold(strings.TrimSpace(item.ProxyId), strings.TrimSpace(proxyId)) {
					dnsServers = item.DnsServers
					break
				}
			}
		}
		return base + "\x00" + dnsServers
	default:
		return ""
	}
}

func selectedEgressForKernel(kernel string, src string, proxies []config.BrowserProxy, proxyId string) (string, outboundEgressMode, string, bool) {
	base := egressBaseKeySourceForKernel(kernel, src, proxies, proxyId)
	if base == "" {
		return "", "", "", false
	}
	return selectedEgressRuntimeKey(base)
}

func appendSelectedEgressLabel(engine string, src string, proxies []config.BrowserProxy, proxyId string) string {
	_, mode, _, ok := selectedEgressForKernel(engine, src, proxies, proxyId)
	if !ok {
		return engine
	}
	label := egressModeLabel(mode)
	if label == "" {
		return engine
	}
	return engine + " · " + label
}

func applySingBoxPhysicalEgress(outbounds []interface{}, interfaceName string) {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return
	}
	for _, raw := range outbounds {
		outbound, ok := raw.(map[string]interface{})
		if !ok || outbound == nil {
			continue
		}
		if detour := strings.TrimSpace(getStringAny(outbound["detour"])); detour != "" {
			continue
		}
		outbound["bind_interface"] = interfaceName
	}
}

func applyXrayPhysicalEgress(outbounds []interface{}, interfaceName string) {
	interfaceName = strings.TrimSpace(interfaceName)
	if interfaceName == "" {
		return
	}
	for _, raw := range outbounds {
		outbound, ok := raw.(map[string]interface{})
		if !ok || outbound == nil {
			continue
		}
		if proxySettings, ok := outbound["proxySettings"].(map[string]interface{}); ok && strings.TrimSpace(getStringAny(proxySettings["tag"])) != "" {
			continue
		}
		stream, _ := outbound["streamSettings"].(map[string]interface{})
		if stream == nil {
			stream = map[string]interface{}{}
			outbound["streamSettings"] = stream
		}
		sockopt, _ := stream["sockopt"].(map[string]interface{})
		if sockopt == nil {
			sockopt = map[string]interface{}{}
			stream["sockopt"] = sockopt
		}
		sockopt["interface"] = interfaceName
	}
}

func probeLocalProxy(ctx context.Context, proxyURL string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return fmt.Errorf("本地代理地址为空")
	}

	var (
		client *http.Client
		err    error
	)
	switch {
	case strings.HasPrefix(strings.ToLower(proxyURL), "socks5://"):
		client, err = buildSocks5HTTPClient(strings.TrimPrefix(proxyURL, "socks5://"), egressProbeTimeout)
	case strings.HasPrefix(strings.ToLower(proxyURL), "http://"), strings.HasPrefix(strings.ToLower(proxyURL), "https://"):
		client, err = buildHTTPProxyClient(proxyURL, egressProbeTimeout)
	default:
		return fmt.Errorf("不支持的本地代理探测地址: %s", proxyURL)
	}
	if err != nil {
		return err
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		defer transport.CloseIdleConnections()
	}

	targets := []string{
		"http://www.gstatic.com/generate_204",
		"https://cp.cloudflare.com/generate_204",
	}
	var lastErr error
	for _, target := range targets {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		resp, reqErr := client.Do(req)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return nil
		}
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("代理出口探测失败")
	}
	return lastErr
}
