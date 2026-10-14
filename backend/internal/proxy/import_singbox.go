package proxy

import (
	"fmt"
	"strings"
	"time"
)

// ImportSingBoxOutbound extracts endpoints, not routing or chained/dialer policy.
// Unsupported endpoint fields fail explicitly instead of silently changing it.
func ImportSingBoxOutbound(out map[string]interface{}) (map[string]interface{}, error) {
	typ := getMapString(out, "type")
	allowed := "type tag server server_port tls"
	switch typ {
	case "http":
		allowed += " username password"
	case "socks":
		allowed += " username password version"
	case "shadowsocks":
		allowed += " method password"
	case "vmess":
		allowed += " uuid security alter_id transport"
	case "vless":
		allowed += " uuid flow transport"
	case "trojan":
		allowed += " password transport"
	case "hysteria2":
		allowed += " password up_mbps down_mbps obfs server_ports"
	case "tuic":
		allowed += " uuid password congestion_control udp_relay_mode zero_rtt_handshake heartbeat"
	case "anytls":
		allowed += " password"
	}
	if err := importKnownFields(out, allowed); err != nil {
		return nil, err
	}
	port := getMapInt(out, "server_port")
	if getMapString(out, "server") == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("sing-box 节点缺少地址或有效端口")
	}
	node := map[string]interface{}{"name": getMapString(out, "tag"), "type": typ, "server": out["server"], "port": port}
	switch typ {
	case "http", "trojan", "hysteria2", "anytls", "tuic":
	case "socks":
		if v := getMapString(out, "version"); v != "" && v != "5" {
			return nil, fmt.Errorf("仅支持 SOCKS5 节点")
		}
		node["type"] = "socks5"
	case "shadowsocks":
		if getMapString(out, "method") == "" {
			return nil, fmt.Errorf("SS 节点缺少加密方式")
		}
		node["type"] = "ss"
		node["cipher"] = out["method"]
	case "vmess":
		node["cipher"] = getMapString(out, "security")
		if node["cipher"] == "" {
			node["cipher"] = "auto"
		}
		node["alterId"] = getMapInt(out, "alter_id")
	case "vless":
	default:
		return nil, fmt.Errorf("不支持的 sing-box 节点类型：%s", typ)
	}
	for _, key := range []string{"username", "password", "uuid", "flow"} {
		if v, ok := out[key]; ok {
			node[key] = v
		}
	}
	if typ == "tuic" && getMapString(node, "password") == "" {
		return nil, fmt.Errorf("TUIC 缺少密码")
	}
	for _, key := range []string{"tls", "transport", "obfs"} {
		if value, exists := out[key]; exists && toStringMap(value) == nil {
			return nil, fmt.Errorf("节点嵌套配置格式无效")
		}
	}
	for source, target := range map[string]string{"up_mbps": "up", "down_mbps": "down", "congestion_control": "congestion-controller", "udp_relay_mode": "udp-relay-mode", "zero_rtt_handshake": "reduce-rtt"} {
		if v, ok := out[source]; ok {
			node[target] = v
		}
	}
	if value, exists := out["heartbeat"]; exists {
		duration, err := time.ParseDuration(fmt.Sprint(value))
		if err != nil || duration <= 0 || duration%time.Millisecond != 0 {
			return nil, fmt.Errorf("TUIC heartbeat 必须为有效的毫秒精度时长")
		}
		node["heartbeat-interval"] = int64(duration / time.Millisecond)
	}
	if _, ok := out["server_ports"]; ok {
		return nil, fmt.Errorf("sing-box server_ports 暂不支持转换，请使用节点链接或 Clash YAML")
	}
	if obfs := toStringMap(out["obfs"]); obfs != nil {
		if err := importKnownFields(obfs, "type password"); err != nil {
			return nil, err
		}
		node["obfs"] = obfs["type"]
		node["obfs-password"] = obfs["password"]
	}
	if tls := toStringMap(out["tls"]); tls != nil {
		if err := importKnownFields(tls, "enabled server_name insecure alpn utls reality"); err != nil {
			return nil, err
		}
		for _, key := range []string{"utls", "reality"} {
			if value, exists := tls[key]; exists && toStringMap(value) == nil {
				return nil, fmt.Errorf("TLS 嵌套配置格式无效")
			}
		}
		if typ == "http" {
			if err := importKnownFields(tls, "enabled"); err != nil {
				return nil, fmt.Errorf("HTTP TLS 的自定义验证参数暂不支持转换")
			}
		}
		if typ == "shadowsocks" || typ == "socks" {
			return nil, fmt.Errorf("此协议的 TLS 包装暂不支持转换")
		}
		if (typ == "trojan" || typ == "hysteria2" || typ == "tuic" || typ == "anytls") && !getMapBool(tls, "enabled") {
			return nil, fmt.Errorf("此协议需要启用 TLS")
		}
		node["tls"] = getMapBool(tls, "enabled")
		node["skip-cert-verify"] = getMapBool(tls, "insecure")
		if s := getMapString(tls, "server_name"); s != "" {
			if typ == "anytls" || typ == "tuic" || typ == "hysteria2" {
				node["sni"] = s
			} else {
				node["servername"] = s
			}
		}
		if v, ok := tls["alpn"]; ok {
			node["alpn"] = v
		}
		if utls := toStringMap(tls["utls"]); utls != nil {
			if err := importKnownFields(utls, "enabled fingerprint"); err != nil {
				return nil, err
			}
			if getMapBool(utls, "enabled") {
				node["client-fingerprint"] = utls["fingerprint"]
			}
		}
		if reality := toStringMap(tls["reality"]); reality != nil {
			if err := importKnownFields(reality, "enabled public_key short_id"); err != nil {
				return nil, err
			}
			if getMapBool(reality, "enabled") {
				if typ != "vless" || getMapString(reality, "public_key") == "" {
					return nil, fmt.Errorf("Reality 需要 VLESS 协议及公钥")
				}
				node["reality-opts"] = map[string]interface{}{"public-key": reality["public_key"], "short-id": reality["short_id"]}
			}
		}
	}
	if transport := toStringMap(out["transport"]); transport != nil {
		if err := importKnownFields(transport, "type path headers service_name"); err != nil {
			return nil, err
		}
		switch getMapString(transport, "type") {
		case "ws":
			node["network"] = "ws"
			node["ws-opts"] = map[string]interface{}{"path": transport["path"], "headers": transport["headers"]}
		case "grpc":
			node["network"] = "grpc"
			node["grpc-opts"] = map[string]interface{}{"grpc-service-name": transport["service_name"]}
		default:
			return nil, fmt.Errorf("sing-box transport 暂不支持转换")
		}
	}
	switch typ {
	case "vmess", "vless", "tuic":
		if getMapString(node, "uuid") == "" {
			return nil, fmt.Errorf("节点缺少 UUID")
		}
	case "trojan", "shadowsocks", "hysteria2", "anytls":
		if getMapString(node, "password") == "" {
			return nil, fmt.Errorf("节点缺少密码")
		}
	}
	return node, nil
}

func importKnownFields(data map[string]interface{}, allowed string) error {
	for key := range data {
		found := false
		for _, field := range strings.Fields(allowed) {
			if field == key {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("存在暂不支持转换的字段（未导入，避免丢失配置）")
		}
	}
	return nil
}
