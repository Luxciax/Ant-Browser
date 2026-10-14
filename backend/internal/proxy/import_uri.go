package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ImportShareURI produces a Clash-compatible node without changing the selected
// connector stack. URI credentials and errors are never echoed to callers.
func ImportShareURI(raw string, index int) (map[string]interface{}, error) {
	raw = normalizeNodeScheme(strings.TrimSpace(raw))
	cut := strings.Index(raw, "://")
	if cut < 1 {
		return nil, fmt.Errorf("需要完整节点链接")
	}
	scheme := strings.ToLower(raw[:cut])
	if scheme == "vmess" {
		return importVmessURI(raw[cut+3:], index)
	}
	if scheme == "ss" {
		node, err := parseSSURIToMapping(raw)
		if err != nil {
			return nil, fmt.Errorf("SS 节点解析失败")
		}
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("SS 链接格式无效")
		}
		if u.Query().Get("plugin") != "" {
			return nil, fmt.Errorf("SS 插件参数暂不支持转换，请导入兼容的 Clash 节点")
		}
		node["name"] = importURIName(u, index)
		return node, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("节点地址格式无效")
	}
	port, err := strconv.Atoi(u.Port())
	if scheme == "hysteria2" && u.Port() == "" {
		port, err = 443, nil
	}
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("节点端口必须为 1–65535；订阅地址请使用 URL 拉取")
	}
	node := map[string]interface{}{"name": importURIName(u, index), "type": scheme, "server": u.Hostname(), "port": port}
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("节点查询参数编码无效")
	}
	switch scheme {
	case "http", "https", "socks5", "socks", "socks5h":
		if scheme == "https" {
			node["type"] = "http"
			node["tls"] = true
		}
		if scheme == "socks" || scheme == "socks5h" {
			node["type"] = "socks5"
		}
		if username != "" {
			node["username"] = username
			node["password"] = password
		}
	case "vless":
		if username == "" {
			return nil, fmt.Errorf("VLESS 缺少 UUID")
		}
		node["uuid"] = username
		if flow := q.Get("flow"); flow != "" {
			node["flow"] = flow
		}
		if encryption := q.Get("encryption"); encryption != "" && encryption != "none" {
			return nil, fmt.Errorf("VLESS encryption 参数暂不支持")
		}
	case "trojan", "hysteria2", "hysteria", "tuic", "anytls":
		if username == "" {
			return nil, fmt.Errorf("节点缺少认证信息")
		}
		node["password"] = username
		if scheme == "tuic" {
			if password == "" {
				return nil, fmt.Errorf("TUIC 缺少密码")
			}
			node["uuid"] = username
			node["password"] = password
		}
		if scheme == "trojan" {
			node["tls"] = true
		}
	default:
		return nil, fmt.Errorf("不支持的节点协议：%s", scheme)
	}
	if sni := firstNonEmptyQueryValue(q, "sni", "peer", "servername"); sni != "" {
		node["servername"] = sni
	}
	if fp := firstNonEmptyQueryValue(q, "fp", "fingerprint", "client-fingerprint"); fp != "" {
		node["client-fingerprint"] = fp
	}
	if scheme == "hysteria2" || scheme == "hysteria" || scheme == "tuic" || scheme == "anytls" {
		if v, ok := node["servername"]; ok {
			node["sni"] = v
			delete(node, "servername")
		}
	}
	if queryBool(q, "insecure", "allowInsecure", "skip-cert-verify") {
		node["skip-cert-verify"] = true
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node["alpn"] = strings.Split(alpn, ",")
	}
	switch strings.ToLower(q.Get("security")) {
	case "", "none":
	case "tls":
		node["tls"] = true
	case "reality":
		node["tls"] = true
		publicKey := firstNonEmptyQueryValue(q, "pbk", "public-key", "publicKey")
		if publicKey == "" {
			return nil, fmt.Errorf("Reality 缺少公钥")
		}
		node["reality-opts"] = map[string]interface{}{"public-key": publicKey, "short-id": firstNonEmptyQueryValue(q, "sid", "short-id", "shortId")}
	default:
		return nil, fmt.Errorf("不支持的 TLS security 参数")
	}
	if q.Get("spx") != "" {
		return nil, fmt.Errorf("Reality spiderX 暂不支持转换")
	}
	if err := importURITransport(node, q); err != nil {
		return nil, err
	}
	if scheme != "vmess" && scheme != "vless" && scheme != "trojan" && getMapString(node, "network") != "" {
		return nil, fmt.Errorf("此协议不支持 WS/gRPC 转换")
	}
	allowed := "sni peer servername insecure allowInsecure skip-cert-verify alpn fp fingerprint client-fingerprint"
	switch scheme {
	case "vless":
		allowed += " flow encryption security pbk public-key publicKey sid short-id shortId spx type network headerType path host serviceName mode"
	case "trojan":
		allowed += " security type network headerType path host serviceName mode"
	case "hysteria2", "hysteria":
		allowed += " obfs obfs-password mport upmbps downmbps"
	case "tuic":
		allowed += " congestion_control udp_relay_mode zero_rtt_handshake"
	}
	queryFields := make(map[string]interface{}, len(q))
	for key := range q {
		queryFields[key] = nil
	}
	if err := importKnownFields(queryFields, allowed); err != nil {
		return nil, err
	}
	if scheme == "http" || scheme == "https" || scheme == "socks5" || scheme == "socks" || scheme == "socks5h" {
		if len(q) != 0 {
			return nil, fmt.Errorf("HTTP/SOCKS5 查询参数暂不支持，请使用标准节点链接")
		}
	}
	if scheme == "hysteria2" || scheme == "hysteria" {
		for queryKey, nodeKey := range map[string]string{"obfs": "obfs", "obfs-password": "obfs-password", "mport": "ports", "upmbps": "up", "downmbps": "down"} {
			if v := q.Get(queryKey); v != "" {
				node[nodeKey] = v
			}
		}
	}
	if scheme == "tuic" {
		for queryKey, nodeKey := range map[string]string{"congestion_control": "congestion-controller", "udp_relay_mode": "udp-relay-mode"} {
			if v := q.Get(queryKey); v != "" {
				node[nodeKey] = v
			}
		}
		if queryBool(q, "zero_rtt_handshake") {
			node["reduce-rtt"] = true
		}
	}
	return node, nil
}

func importURIName(u *url.URL, index int) string {
	if strings.TrimSpace(u.Fragment) != "" {
		return strings.TrimSpace(u.Fragment)
	}
	return fmt.Sprintf("导入代理 %d", index+1)
}

func importURITransport(node map[string]interface{}, q url.Values) error {
	network := firstNonEmptyQueryValue(q, "type", "network")
	switch network {
	case "", "tcp":
		if header := q.Get("headerType"); header != "" && header != "none" {
			return fmt.Errorf("TCP headerType 暂不支持转换")
		}
	case "ws":
		node["network"] = "ws"
		options := map[string]interface{}{"path": q.Get("path")}
		if host := q.Get("host"); host != "" {
			options["headers"] = map[string]interface{}{"Host": host}
		}
		node["ws-opts"] = options
	case "grpc":
		node["network"] = "grpc"
		node["grpc-opts"] = map[string]interface{}{"grpc-service-name": q.Get("serviceName")}
		if mode := q.Get("mode"); mode != "" && mode != "gun" {
			return fmt.Errorf("gRPC mode 暂不支持转换")
		}
	default:
		return fmt.Errorf("不支持的传输类型：%s", network)
	}
	return nil
}

func importVmessURI(encoded string, index int) (map[string]interface{}, error) {
	data, err := decodeBase64String(encoded)
	if err != nil {
		return nil, fmt.Errorf("VMess Base64 无效")
	}
	var v map[string]interface{}
	if json.Unmarshal(data, &v) != nil {
		return nil, fmt.Errorf("VMess JSON 无效")
	}
	if err := importKnownFields(v, "v ps add port id aid scy net type host path tls sni fp alpn"); err != nil {
		return nil, err
	}
	if tls := getMapString(v, "tls"); tls != "" && tls != "tls" && tls != "none" {
		return nil, fmt.Errorf("VMess TLS 类型暂不支持转换")
	}
	port := getMapInt(v, "port")
	if getMapString(v, "add") == "" || port < 1 || port > 65535 || getMapString(v, "id") == "" {
		return nil, fmt.Errorf("VMess 缺少地址、有效端口或 UUID")
	}
	name := getMapString(v, "ps")
	if name == "" {
		name = fmt.Sprintf("导入代理 %d", index+1)
	}
	cipher := getMapString(v, "scy")
	if cipher == "" {
		cipher = "auto"
	}
	node := map[string]interface{}{"name": name, "type": "vmess", "server": getMapString(v, "add"), "port": port, "uuid": getMapString(v, "id"), "alterId": getMapInt(v, "aid"), "cipher": cipher}
	if getMapString(v, "tls") == "tls" {
		node["tls"] = true
	}
	for source, target := range map[string]string{"sni": "servername", "fp": "client-fingerprint"} {
		if s := getMapString(v, source); s != "" {
			node[target] = s
		}
	}
	if s := getMapString(v, "alpn"); s != "" {
		node["alpn"] = strings.Split(s, ",")
	}
	q := url.Values{"type": {getMapString(v, "net")}, "path": {getMapString(v, "path")}, "host": {getMapString(v, "host")}}
	if q.Get("type") == "grpc" {
		q.Set("serviceName", getMapString(v, "path"))
	}
	if header := getMapString(v, "type"); header != "" && header != "none" {
		return nil, fmt.Errorf("VMess header type 暂不支持转换")
	}
	if err := importURITransport(node, q); err != nil {
		return nil, err
	}
	return node, nil
}
