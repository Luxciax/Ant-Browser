package backend

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type remoteIPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type safeRemoteRoundTripper struct {
	template *http.Transport
	resolver remoteIPResolver
}

func newSafeRemoteHTTPClient(base *http.Client, timeout time.Duration) (*http.Client, error) {
	return newSafeRemoteHTTPClientWithResolver(base, timeout, net.DefaultResolver)
}

func newSafeRemoteHTTPClientWithResolver(base *http.Client, timeout time.Duration, resolver remoteIPResolver) (*http.Client, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if base == nil {
		base = &http.Client{}
	}

	var transport *http.Transport
	switch current := base.Transport.(type) {
	case nil:
		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("默认 HTTP Transport 类型不受支持")
		}
		transport = defaultTransport.Clone()
	case *http.Transport:
		transport = current.Clone()
	default:
		return nil, fmt.Errorf("远程导入 HTTP Transport 类型不受支持")
	}

	client := *base
	if timeout > 0 {
		client.Timeout = timeout
	}
	client.Transport = &safeRemoteRoundTripper{template: transport, resolver: resolver}
	return &client, nil
}

func (t *safeRemoteRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("远程请求地址为空")
	}
	if err := validateRemoteHTTPURL(req.URL); err != nil {
		return nil, err
	}

	hostname := strings.TrimSpace(req.URL.Hostname())
	ip, err := resolveSafeRemoteIP(req.Context(), hostname, t.resolver)
	if err != nil {
		return nil, err
	}
	port := strings.TrimSpace(req.URL.Port())
	if port == "" {
		if strings.EqualFold(req.URL.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}

	transport := t.template.Clone()
	transport.DisableKeepAlives = true
	if strings.EqualFold(req.URL.Scheme, "https") {
		var tlsConfig *tls.Config
		if transport.TLSClientConfig != nil {
			tlsConfig = transport.TLSClientConfig.Clone()
		} else {
			tlsConfig = &tls.Config{}
		}
		tlsConfig.ServerName = hostname
		transport.TLSClientConfig = tlsConfig
	}

	cloned := req.Clone(req.Context())
	urlCopy := *req.URL
	cloned.URL = &urlCopy
	cloned.Host = req.URL.Host
	cloned.URL.Host = net.JoinHostPort(ip.String(), port)
	return transport.RoundTrip(cloned)
}

func validateRemoteHTTPURL(parsed *url.URL) error {
	if parsed == nil {
		return fmt.Errorf("远程 URL 为空")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("远程 URL 仅支持 http/https")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("远程 URL 缺少主机名")
	}
	if parsed.User != nil {
		return fmt.Errorf("远程 URL 不允许包含内嵌凭据")
	}
	return nil
}

func resolveSafeRemoteIP(ctx context.Context, hostname string, resolver remoteIPResolver) (net.IP, error) {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil, fmt.Errorf("远程 URL 缺少主机名")
	}
	if literal := net.ParseIP(hostname); literal != nil {
		if isForbiddenRemoteIP(literal) {
			return nil, fmt.Errorf("拒绝访问本地或私有网络地址 %s", literal.String())
		}
		return literal, nil
	}

	addresses, err := resolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("解析远程主机 %s 失败: %w", hostname, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("远程主机 %s 没有可用 IP", hostname)
	}

	var selected net.IP
	for _, address := range addresses {
		ip := address.IP
		if isForbiddenRemoteIP(ip) {
			return nil, fmt.Errorf("远程主机 %s 解析到本地或私有地址 %s，已拒绝访问", hostname, ip.String())
		}
		if selected == nil {
			selected = append(net.IP(nil), ip...)
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("远程主机 %s 没有可用公网 IP", hostname)
	}
	return selected, nil
}

func isForbiddenRemoteIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast()
}
