package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type portableSessionStorage struct {
	Origin  string      `json:"origin"`
	Entries [][2]string `json:"entries"`
}
type portablePageSession struct {
	URL     string                   `json:"url"`
	Storage []portableSessionStorage `json:"storage,omitempty"`
}

type loginPageCDP struct {
	conn     *websocket.Conn
	sequence int
	deadline time.Time
}

func dialLoginPage(ws string) (*loginPageCDP, error) {
	conn, err := cdpDialWebSocket(ws)
	if err != nil {
		return nil, fmt.Errorf("页面调试连接不可用")
	}
	return &loginPageCDP{conn: conn}, nil
}
func (s *loginPageCDP) call(method string, params map[string]any) (map[string]any, error) {
	s.sequence++
	deadline := time.Now().Add(3 * time.Second)
	if !s.deadline.IsZero() && s.deadline.Before(deadline) {
		deadline = s.deadline
	}
	if !deadline.After(time.Now()) {
		return nil, fmt.Errorf("页面快照读取超时")
	}
	_ = s.conn.SetWriteDeadline(deadline)
	if err := s.conn.WriteJSON(cdpMessage{Id: s.sequence, Method: method, Params: params}); err != nil {
		return nil, fmt.Errorf("页面调试命令发送失败")
	}
	_ = s.conn.SetReadDeadline(deadline)
	response, err := readCDPResponseForID(s.conn, s.sequence)
	if err != nil || response.Error != nil {
		return nil, fmt.Errorf("页面调试命令失败（%s）", method)
	}
	return response.Result, nil
}

func webOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func capturePageSession(target cdpTarget) (portablePageSession, int) {
	page := portablePageSession{URL: target.URL}
	session, err := dialLoginPage(target.WebSocketDebuggerUrl)
	if err != nil {
		return page, 1
	}
	defer session.conn.Close()
	session.deadline = time.Now().Add(3 * time.Second)
	tree, err := session.call("Page.getFrameTree", nil)
	if err != nil {
		return page, 1
	}
	seen := map[string]bool{}
	skipped := 0
	var visit func(map[string]any)
	visit = func(node map[string]any) {
		frame, _ := node["frame"].(map[string]any)
		origin, _ := frame["securityOrigin"].(string)
		if webOrigin(origin) != "" && !seen[origin] {
			seen[origin] = true
			result, err := session.call("DOMStorage.getDOMStorageItems", map[string]any{"storageId": map[string]any{"securityOrigin": origin, "isLocalStorage": false}})
			if err != nil {
				skipped++
			} else {
				data, _ := json.Marshal(result["entries"])
				var items [][2]string
				if len(data) > 4<<20 || json.Unmarshal(data, &items) != nil {
					skipped++
				} else if len(items) > 0 {
					page.Storage = append(page.Storage, portableSessionStorage{origin, items})
				}
				zeroSensitiveBytes(data)
			}
		}
		children, _ := node["childFrames"].([]any)
		for _, child := range children {
			if child, ok := child.(map[string]any); ok {
				visit(child)
			}
		}
	}
	if root, ok := tree["frameTree"].(map[string]any); ok {
		visit(root)
	} else {
		skipped++
	}
	return page, skipped
}

func sessionRestoreSource(page portablePageSession) (string, error) {
	byOrigin := map[string][][2]string{}
	for _, storage := range page.Storage {
		byOrigin[storage.Origin] = storage.Entries
	}
	data, err := json.Marshal(byOrigin)
	if err != nil {
		return "", fmt.Errorf("会话存储恢复脚本编码失败")
	}
	// This runs before the first website script, in each matching origin only.
	// No global markers/bindings are installed and no credentials are logged.
	return `(function antRestoreSessionStorage(){const items=(` + string(data) + `)[location.origin];if(items)for(const [k,v] of items)sessionStorage.setItem(k,v);})();` + "\n//# sourceURL=ant-session-restore.js", nil
}

func resolveLoginPageTarget(port int, id string) (cdpTarget, error) {
	for attempt := 0; attempt < 10; attempt++ {
		body, err := cdpGetEndpointBody(port, "/json/list")
		if err != nil {
			return cdpTarget{}, fmt.Errorf("新标签页调试目标不可用")
		}
		var targets []cdpTarget
		if json.Unmarshal(body, &targets) != nil {
			return cdpTarget{}, fmt.Errorf("标签页调试目标格式无效")
		}
		for _, target := range targets {
			if target.ID == id {
				return target, nil
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return cdpTarget{}, fmt.Errorf("新标签页调试目标尚未就绪")
}

func openPageWithSessionStorage(ctx context.Context, port int, page portablePageSession) (resultErr error) {
	created, err := cdpBrowserCallResult(port, "Target.createTarget", map[string]any{"url": "about:blank"})
	if err != nil {
		return fmt.Errorf("会话恢复标签页创建失败")
	}
	id, _ := created["targetId"].(string)
	target, err := resolveLoginPageTarget(port, id)
	if err != nil {
		return err
	}
	session, err := dialLoginPage(target.WebSocketDebuggerUrl)
	if err != nil {
		return err
	}
	defer session.conn.Close()
	if _, err := session.call("Page.enable", nil); err != nil {
		return err
	}
	if _, err := session.call("Runtime.enable", nil); err != nil {
		return err
	}
	if _, err := session.call("Page.setLifecycleEventsEnabled", map[string]any{"enabled": true}); err != nil {
		return err
	}
	source, err := sessionRestoreSource(page)
	if err != nil {
		return err
	}
	installed, err := session.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": source})
	if err != nil {
		return err
	}
	identifier, _ := installed["identifier"].(string)
	if identifier == "" {
		return fmt.Errorf("浏览器没有返回一次性恢复脚本标识")
	}
	defer func() {
		if _, err := session.call("Page.removeScriptToEvaluateOnNewDocument", map[string]any{"identifier": identifier}); err != nil && resultErr == nil {
			resultErr = fmt.Errorf("一次性会话恢复脚本清理未确认，请检查实例状态")
		}
	}()
	// Send navigation without swallowing DOMContentLoaded events in the command
	// response reader. Wait for BOTH the response and document readiness.
	session.sequence++
	navigationID := session.sequence
	if err := session.conn.WriteJSON(cdpMessage{Id: navigationID, Method: "Page.navigate", Params: map[string]any{"url": page.URL}}); err != nil {
		return fmt.Errorf("会话恢复页面导航失败")
	}
	deadline := time.Now().Add(5 * time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	_ = session.conn.SetReadDeadline(deadline)
	responseReady, documentReady := false, false
	loader := ""
	readyLoaders := map[string]bool{}
	for !responseReady || !documentReady {
		if ctx.Err() != nil {
			return fmt.Errorf("会话恢复超过本次启动时间窗口")
		}
		var message struct {
			Id     int             `json:"id"`
			Method string          `json:"method"`
			Result map[string]any  `json:"result"`
			Params map[string]any  `json:"params"`
			Error  json.RawMessage `json:"error"`
		}
		if session.conn.ReadJSON(&message) != nil {
			return fmt.Errorf("会话存储页面未按时就绪；未确认完成")
		}
		if message.Method == "Runtime.exceptionThrown" {
			if reason := sessionStorageExceptionReason(message.Params); reason != "" {
				return fmt.Errorf("会话存储写入失败（%s），未确认完成", reason)
			}
		}
		if message.Id == navigationID {
			errorText, _ := message.Result["errorText"].(string)
			if len(message.Error) > 0 && string(message.Error) != "null" || errorText != "" {
				return fmt.Errorf("会话恢复页面导航失败；未确认完成")
			}
			responseReady = true
			loader, _ = message.Result["loaderId"].(string)
		}
		if message.Method == "Page.lifecycleEvent" && message.Params["name"] == "DOMContentLoaded" {
			if id, ok := message.Params["loaderId"].(string); ok {
				readyLoaders[id] = true
			}
		}
		documentReady = loader != "" && readyLoaders[loader]
	}
	return nil
}

func sessionStorageExceptionReason(params map[string]any) string {
	details, _ := params["exceptionDetails"].(map[string]any)
	ours := details["url"] == "ant-session-restore.js"
	stack, _ := details["stackTrace"].(map[string]any)
	frames, _ := stack["callFrames"].([]any)
	for _, frame := range frames {
		if fields, ok := frame.(map[string]any); ok && (fields["functionName"] == "antRestoreSessionStorage" || fields["url"] == "ant-session-restore.js") {
			ours = true
		}
	}
	if !ours {
		return ""
	}
	exception, _ := details["exception"].(map[string]any)
	text, _ := exception["description"].(string)
	if strings.Contains(text, "SecurityError") {
		return "permission_denied"
	}
	if strings.Contains(text, "QuotaExceededError") {
		return "quota_exceeded"
	}
	return "script_error"
}

func openLoginRestoreTargets(ctx context.Context, port int, state portableLoginState, targets []string) error {
	used := make([]bool, len(state.Pages))
	for _, target := range targets {
		if ctx.Err() != nil {
			return fmt.Errorf("登录态恢复启动页超过本次启动时间窗口")
		}
		matched := -1
		for i, page := range state.Pages {
			if !used[i] && page.URL == target {
				matched = i
				used[i] = true
				break
			}
		}
		if matched >= 0 && len(state.Pages[matched].Storage) > 0 {
			if err := openPageWithSessionStorage(ctx, port, state.Pages[matched]); err != nil {
				return err
			}
		} else if err := createBrowserStartTarget(port, target); err != nil {
			return fmt.Errorf("登录态恢复启动页打开失败")
		}
	}
	return nil
}
