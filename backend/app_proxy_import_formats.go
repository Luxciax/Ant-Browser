package backend

import (
	"ant-chrome/backend/internal/proxy"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// BrowserProxyParseImport shares the subscription-refresh parser with pasted
// content. It only previews nodes; saving remains an explicit UI action.
func (a *App) BrowserProxyParseImport(raw string) (map[string]interface{}, error) {
	content, payload, err := normalizeClashSubscriptionContent([]byte(raw))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"content": content, "proxyCount": clashProxyCount(payload), "dnsServers": extractClashDNSYAML(payload)}, nil
}

func normalizeProxyImportText(text string) (string, interface{}, error) {
	var nodes []interface{}
	var result map[string]interface{}
	singbox := false
	if payload, ok := parseClashPayload(text); ok {
		if root := toStringMap(payload); root != nil {
			result = root
			if out, ok := root["outbounds"].([]interface{}); ok {
				nodes = out
				singbox = true
				result = nil
			} else if list, ok := root["proxies"].([]interface{}); ok {
				nodes = list
			} else if list, ok := root["proxy"].([]interface{}); ok {
				nodes = list
			} else if list, ok := root["Proxy"].([]interface{}); ok {
				nodes = list
			} else if _, ok := root["server"]; ok {
				nodes = []interface{}{root}
				result = nil
			}
		} else if list, ok := payload.([]interface{}); ok {
			nodes = list
		}
	}
	if nodes != nil {
		normalized := make([]interface{}, 0, len(nodes))
		for index, value := range nodes {
			node := toStringMap(value)
			if node == nil {
				return "", nil, fmt.Errorf("第 %d 个节点不是有效配置对象", index+1)
			}
			if singbox {
				switch getMapString(node, "type") {
				case "direct", "block", "reject", "dns", "selector", "urltest":
					continue
				}
			}
			if _, isSingBox := node["server_port"]; singbox || isSingBox {
				var err error
				node, err = proxy.ImportSingBoxOutbound(node)
				if err != nil {
					return "", nil, fmt.Errorf("第 %d 个节点：%w", index+1, err)
				}
			}
			port, err := strconv.Atoi(getMapString(node, "port"))
			if getMapString(node, "type") == "" || getMapString(node, "server") == "" || err != nil || port < 1 || port > 65535 {
				return "", nil, fmt.Errorf("第 %d 个节点缺少协议、地址或有效端口", index+1)
			}
			node["port"] = port
			if strings.TrimSpace(getMapString(node, "name")) == "" {
				node["name"] = fmt.Sprintf("导入代理 %d", index+1)
			}
			normalized = append(normalized, node)
		}
		if len(normalized) == 0 {
			return "", nil, fmt.Errorf("未检测到可导入的代理节点")
		}
		if result == nil {
			result = map[string]interface{}{}
		}
		delete(result, "proxy")
		delete(result, "Proxy")
		result["proxies"] = normalized
		return marshalProxyImport(result)
	}
	list := make([]interface{}, 0)
	for index, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		node, err := proxy.ImportShareURI(line, index)
		if err != nil {
			return "", nil, fmt.Errorf("第 %d 行：%w", index+1, err)
		}
		list = append(list, node)
	}
	if len(list) == 0 {
		return "", nil, fmt.Errorf("支持节点链接、Base64 订阅、Clash YAML/JSON 和 sing-box JSON")
	}
	return marshalProxyImport(map[string]interface{}{"proxies": list})
}

func marshalProxyImport(payload map[string]interface{}) (string, interface{}, error) {
	data, err := yaml.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("节点配置转换失败")
	}
	return strings.TrimSpace(string(data)), payload, nil
}
