package backend

import (
	"ant-chrome/backend/internal/fsutil"
	"ant-chrome/backend/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const loginSnapshotFile = "ant-login-snapshot.bin"
const loginRestoreFile = "ant-login-restore.bin"
const loginStateMaxBytes = 32 << 20

// Only serializable cookie input fields are retained. CDP output-only fields
// (size, session, partitionKeyOpaque) must never be passed to setCookies.
type portableCookie struct {
	Name         string          `json:"name"`
	Value        string          `json:"value"`
	Domain       string          `json:"domain"`
	Path         string          `json:"path"`
	Expires      float64         `json:"expires,omitempty"`
	HTTPOnly     bool            `json:"httpOnly"`
	Secure       bool            `json:"secure"`
	SameSite     string          `json:"sameSite,omitempty"`
	Priority     string          `json:"priority,omitempty"`
	SourceScheme string          `json:"sourceScheme,omitempty"`
	SourcePort   *int            `json:"sourcePort,omitempty"`
	PartitionKey json.RawMessage `json:"partitionKey,omitempty"`
}

type portableLoginState struct {
	Version    int              `json:"version"`
	CapturedAt time.Time        `json:"capturedAt"`
	Cookies    []portableCookie `json:"cookies"`
	Tabs       []string         `json:"tabs,omitempty"`
}

func capturePortableLoginState(port int) (portableLoginState, error) {
	result, err := cdpBrowserCallResult(port, "Storage.getCookies", nil)
	if err != nil {
		return portableLoginState{}, fmt.Errorf("读取浏览器 Cookie 失败（请保持实例可用后重试）")
	}
	data, err := json.Marshal(result["cookies"])
	if err != nil {
		return portableLoginState{}, fmt.Errorf("Cookie 快照编码失败")
	}
	defer zeroSensitiveBytes(data)
	if len(data) > loginStateMaxBytes {
		return portableLoginState{}, fmt.Errorf("Cookie 快照超过 32 MiB 限制")
	}
	var cookies []portableCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return portableLoginState{}, fmt.Errorf("Cookie 快照解析失败")
	}
	var raw []struct {
		Opaque bool `json:"partitionKeyOpaque"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return portableLoginState{}, fmt.Errorf("Cookie 分区信息解析失败")
	}
	for _, cookie := range raw {
		if cookie.Opaque {
			return portableLoginState{}, fmt.Errorf("浏览器包含无法迁移的不透明分区 Cookie，未生成可能降级分区的快照")
		}
	}
	// Session cookies have expires=-1 in CDP output; omit expiration on input.
	for i := range cookies {
		if cookies[i].Expires < 0 {
			cookies[i].Expires = 0
		}
	}
	state := portableLoginState{Version: 2, CapturedAt: time.Now().UTC(), Cookies: cookies}
	if body, listErr := cdpGetEndpointBody(port, "/json/list"); listErr == nil {
		var targets []cdpTarget
		if json.Unmarshal(body, &targets) == nil {
			for _, target := range targets {
				parsed, parseErr := url.Parse(target.URL)
				if target.Type == "page" && parseErr == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
					state.Tabs = append(state.Tabs, target.URL)
				}
			}
		}
	}
	return state, validatePortableLoginState(state)
}

func validatePortableLoginState(state portableLoginState) error {
	if state.Version != 2 || state.CapturedAt.IsZero() || len(state.Cookies) > 100000 {
		return fmt.Errorf("登录态快照格式或数量无效")
	}
	if len(state.Tabs) > 10000 {
		return fmt.Errorf("登录态标签页数量无效")
	}
	for _, tab := range state.Tabs {
		parsed, err := url.Parse(tab)
		if err != nil || parsed.Host == "" || parsed.Scheme != "https" && parsed.Scheme != "http" {
			return fmt.Errorf("登录态标签页地址无效")
		}
	}
	for _, cookie := range state.Cookies {
		if cookie.Domain == "" || cookie.Path == "" || len(cookie.Name)+len(cookie.Value) > 1<<20 {
			return fmt.Errorf("登录态 Cookie 字段无效")
		}
		if cookie.SameSite != "" && cookie.SameSite != "Strict" && cookie.SameSite != "Lax" && cookie.SameSite != "None" {
			return fmt.Errorf("登录态 SameSite 字段无效")
		}
		if len(cookie.PartitionKey) > 0 && string(cookie.PartitionKey) != "null" {
			var key struct {
				TopLevelSite         string `json:"topLevelSite"`
				HasCrossSiteAncestor *bool  `json:"hasCrossSiteAncestor"`
			}
			if json.Unmarshal(cookie.PartitionKey, &key) != nil || !strings.HasPrefix(key.TopLevelSite, "https://") && !strings.HasPrefix(key.TopLevelSite, "http://") || key.HasCrossSiteAncestor == nil {
				return fmt.Errorf("登录态分区 Cookie 字段无效")
			}
		}
	}
	return nil
}

func writeLocalLoginState(dir, name string, state portableLoginState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("登录态快照编码失败")
	}
	defer zeroSensitiveBytes(data)
	protected, err := protectProfileOSCryptKey(data)
	if err != nil {
		return fmt.Errorf("登录态快照本机加密失败")
	}
	defer zeroSensitiveBytes(protected)
	return fsutil.AtomicWriteFile(filepath.Join(dir, name), protected, 0600)
}

func readLocalLoginState(dir, name string) (portableLoginState, error) {
	file, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return portableLoginState{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return portableLoginState{}, err
	}
	if info.Size() > loginStateMaxBytes {
		return portableLoginState{}, fmt.Errorf("本机登录态快照过大")
	}
	protected, err := io.ReadAll(io.LimitReader(file, loginStateMaxBytes+1))
	if err != nil {
		return portableLoginState{}, err
	}
	if len(protected) > loginStateMaxBytes {
		return portableLoginState{}, fmt.Errorf("本机登录态快照过大")
	}
	defer zeroSensitiveBytes(protected)
	data, err := unprotectProfileOSCryptKey(protected)
	if err != nil {
		return portableLoginState{}, fmt.Errorf("本机登录态快照无法解密；请在原设备打开实例后正常停止，再重新导出")
	}
	defer zeroSensitiveBytes(data)
	var state portableLoginState
	if json.Unmarshal(data, &state) != nil {
		return state, fmt.Errorf("本机登录态快照格式无效")
	}
	return state, validatePortableLoginState(state)
}

func snapshotLoginForBackup(port int, dir, profileID string) error {
	state, err := capturePortableLoginState(port)
	if err == nil {
		err = writeLocalLoginState(dir, loginSnapshotFile, state)
	}
	log := logger.New("LoginMigration")
	if err != nil {
		// Never include CDP response bodies, cookie names/values or origins.
		log.Warn("登录态快照失败", logger.F("event", "login.snapshot_failed"), logger.F("profile_id", profileID), logger.F("reason", err.Error()))
		return err
	}
	log.Info("登录态快照已加密", logger.F("event", "login.snapshot_saved"), logger.F("profile_id", profileID), logger.F("cookie_count", len(state.Cookies)), logger.F("captured_at", state.CapturedAt))
	return nil
}

func (a *App) observeLoginSnapshots(ctx context.Context, port int, dir, profileID string) {
	if runtime.GOOS != "windows" {
		return
	}
	// Use the existing per-instance lifecycle, not a new resident service.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		unlock := a.lockProfileRuntimeOperation(profileID)
		if ctx.Err() != nil {
			unlock()
			return
		}
		_ = snapshotLoginForBackup(port, dir, profileID)
		unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func restorePortableCookies(port int, state portableLoginState) (int, error) {
	cookies := make([]portableCookie, 0, len(state.Cookies))
	now := float64(time.Now().Unix())
	for _, cookie := range state.Cookies {
		if cookie.Expires == 0 || cookie.Expires > now {
			cookies = append(cookies, cookie)
		}
	}
	if len(cookies) > 0 {
		if err := cdpBrowserCall(port, "Storage.setCookies", map[string]any{"cookies": cookies}); err != nil {
			return 0, fmt.Errorf("浏览器拒绝写入迁移 Cookie，恢复文件已保留；请使用兼容内核重试")
		}
	}
	readBack, err := capturePortableLoginState(port)
	if err != nil {
		return 0, fmt.Errorf("迁移 Cookie 写入后回读失败，恢复文件已保留")
	}
	lookup := map[string]portableCookie{}
	for _, cookie := range readBack.Cookies {
		lookup[portableCookieID(cookie)] = cookie
	}
	for _, cookie := range cookies {
		got, ok := lookup[portableCookieID(cookie)]
		if !ok || got.Value != cookie.Value || got.Secure != cookie.Secure || got.HTTPOnly != cookie.HTTPOnly || got.SameSite != cookie.SameSite {
			return 0, fmt.Errorf("迁移 Cookie 回读不匹配，恢复文件已保留；未确认恢复成功")
		}
	}
	return len(cookies), nil
}

func portableCookieID(cookie portableCookie) string {
	var key any
	_ = json.Unmarshal(cookie.PartitionKey, &key)
	canonical, _ := json.Marshal(key)
	return cookie.Domain + "\x00" + cookie.Path + "\x00" + cookie.Name + "\x00" + string(canonical)
}

func prepareSameDeviceLogin(dir string) (bool, error) {
	state, err := readLocalLoginState(dir, loginSnapshotFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, writeLocalLoginState(dir, loginRestoreFile, state)
}
