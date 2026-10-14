// Package identity treats persisted launch arguments as the single source of
// identity. Sampling and alignment are explicit edits, never startup resampling.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type Geo struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"`
}

type Identity struct {
	Seed                string   `json:"seed"`
	Platform            string   `json:"platform"`
	PlatformVersion     string   `json:"platformVersion"`
	Brand               string   `json:"brand"`
	BrandVersion        string   `json:"brandVersion"`
	UserAgent           string   `json:"userAgent"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	WindowSize          string   `json:"windowSize"`
	Locale              string   `json:"locale"`
	Languages           []string `json:"languages"`
	Timezone            string   `json:"timezone"`
	Geo                 *Geo     `json:"geo,omitempty"`
	TemplateID          string   `json:"templateId,omitempty"`
	FontPolicy          string   `json:"fontPolicy"`
}

type Issue struct {
	Field   string `json:"field"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Template struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platformVersion"`
	CPU             int    `json:"cpu"`
	WindowSize      string `json:"windowSize"`
}

func Templates() []Template {
	var out []Template
	for _, p := range []struct{ id, name, version string }{{"windows", "Windows", "10.0.0"}, {"macos", "macOS", "15.2.0"}, {"linux", "Linux", ""}} {
		for _, cpu := range []int{4, 8} {
			for _, size := range []string{"1366,768", "1920,1080"} {
				out = append(out, Template{fmt.Sprintf("%s-%d-%s", p.id, cpu, strings.ReplaceAll(size, ",", "x")), fmt.Sprintf("%s · %d 核 · %s", p.name, cpu, strings.ReplaceAll(size, ",", " × ")), p.id, p.version, cpu, size})
			}
		}
	}
	return out
}

func Value(args []string, key string) string {
	for n := len(args) - 1; n >= 0; n-- {
		if k, v, ok := strings.Cut(strings.TrimSpace(args[n]), "="); ok && strings.EqualFold(k, key) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func Set(args []string, key, value string) []string {
	out := make([]string, 0, len(args)+1)
	for _, arg := range args {
		k, _, _ := strings.Cut(strings.TrimSpace(arg), "=")
		if !strings.EqualFold(k, key) {
			out = append(out, arg)
		}
	}
	if value != "" {
		out = append(out, key+"="+value)
	}
	return out
}

func FromArgs(args []string) Identity {
	i := Identity{Seed: Value(args, "--fingerprint"), Platform: strings.ToLower(Value(args, "--fingerprint-platform")), PlatformVersion: Value(args, "--fingerprint-platform-version"), Brand: Value(args, "--fingerprint-brand"), BrandVersion: Value(args, "--fingerprint-brand-version"), UserAgent: Value(args, "--user-agent"), WindowSize: Value(args, "--window-size"), Locale: Value(args, "--lang"), Timezone: Value(args, "--timezone"), TemplateID: Value(args, "--ant-identity-template"), FontPolicy: Value(args, "--ant-font-policy")}
	i.HardwareConcurrency, _ = strconv.Atoi(Value(args, "--fingerprint-hardware-concurrency"))
	switch i.Platform {
	case "win":
		i.Platform = "windows"
	case "mac", "darwin":
		i.Platform = "macos"
	}
	if v := Value(args, "--accept-lang"); v != "" {
		i.Languages = strings.Split(v, ",")
	}
	if v := Value(args, "--ant-geolocation"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) == 3 {
			lat, e1 := strconv.ParseFloat(parts[0], 64)
			lon, e2 := strconv.ParseFloat(parts[1], 64)
			acc, e3 := strconv.ParseFloat(parts[2], 64)
			if e1 == nil && e2 == nil && e3 == nil {
				i.Geo = &Geo{lat, lon, acc}
			}
		}
	}
	return i
}

func (i Identity) Hash() string {
	i.TemplateID = "" // provenance must not make identical devices look unique
	b, _ := json.Marshal(i)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

var chromeVersion = regexp.MustCompile(`Chrome/(\d+)`)

func Major(v string) int {
	n, _ := strconv.Atoi(strings.Split(strings.TrimSpace(v), ".")[0])
	return n
}

func ReducedUA(platform string, major int) string {
	var osPart string
	switch platform {
	case "windows":
		osPart = "Windows NT 10.0; Win64; x64"
	case "macos":
		osPart = "Macintosh; Intel Mac OS X 10_15_7"
	case "linux":
		osPart = "X11; Linux x86_64"
	default:
		return ""
	}
	return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36", osPart, major)
}

func Validate(i Identity, kernelVersion string) []Issue {
	out := make([]Issue, 0)
	add := func(field, level, msg string) { out = append(out, Issue{field, level, msg}) }
	if i.Seed != "" {
		if n, err := strconv.ParseUint(i.Seed, 10, 31); err != nil || n == 0 {
			add("seed", "error", "指纹种子须为 1–2147483647 的整数")
		}
	}
	if i.Platform != "" && ReducedUA(i.Platform, 1) == "" {
		add("platform", "error", "身份平台仅支持 windows、macos、linux")
	}
	if i.HardwareConcurrency < 0 || i.HardwareConcurrency > 128 {
		add("hardwareConcurrency", "error", "CPU 核心数须在 1–128 范围内")
	}
	if i.Locale != "" && len(i.Languages) > 0 && !strings.EqualFold(i.Locale, strings.TrimSpace(strings.Split(i.Languages[0], ";")[0])) {
		add("locale", "warning", "主语言与语言列表第一项不一致")
	}
	if i.Timezone != "" && i.Timezone != "system" {
		if _, err := time.LoadLocation(i.Timezone); err != nil {
			add("timezone", "error", "时区不是有效的 IANA 名称")
		}
	}
	if i.FontPolicy != "" && i.FontPolicy != "spoof" && i.FontPolicy != "host" {
		add("fontPolicy", "error", "字体策略须为 spoof 或 host")
	}
	if i.WindowSize != "" {
		p := strings.Split(i.WindowSize, ",")
		valid := len(p) == 2
		for _, v := range p {
			n, e := strconv.Atoi(v)
			if e != nil || n < 100 || n > 16384 {
				valid = false
			}
		}
		if !valid {
			add("windowSize", "error", "窗口尺寸须为宽,高（100–16384）")
		}
	}
	if i.Geo != nil && !ValidGeo(*i.Geo) {
		add("geo", "error", "定位坐标或精度无效")
	}
	if ua := i.UserAgent; ua != "" {
		expected := map[string]string{"windows": "Windows NT", "macos": "Macintosh", "linux": "Linux"}[i.Platform]
		if expected != "" && !strings.Contains(ua, expected) {
			add("userAgent", "warning", "UA 平台与身份平台不一致")
		}
		if parts := chromeVersion.FindStringSubmatch(ua); len(parts) > 1 && Major(parts[1]) != Major(kernelVersion) && Major(kernelVersion) > 0 {
			add("userAgent", "warning", "UA 大版本与实际内核不一致")
		}
	}
	if i.BrandVersion != "" && Major(kernelVersion) > 0 && Major(i.BrandVersion) != Major(kernelVersion) {
		add("brandVersion", "warning", "Client Hints 品牌大版本与实际内核不一致")
	}
	if i.Brand != "" && !strings.EqualFold(i.Brand, "Chrome") {
		add("brand", "warning", "身份模板按 Chromium/Chrome 建模；其他品牌的能力不能仅靠 UA 保证")
	}
	return out
}

func ValidGeo(g Geo) bool {
	return !math.IsNaN(g.Latitude) && !math.IsNaN(g.Longitude) && !math.IsNaN(g.Accuracy) && !math.IsInf(g.Latitude, 0) && !math.IsInf(g.Longitude, 0) && !math.IsInf(g.Accuracy, 0) && math.Abs(g.Latitude) <= 90 && math.Abs(g.Longitude) <= 180 && g.Accuracy > 0
}

func ApplyTemplate(args []string, templateID, stableSeed, kernelVersion string) ([]string, error) {
	var chosen *Template
	for _, t := range Templates() {
		if t.ID == templateID {
			copy := t
			chosen = &copy
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("未找到身份模板 %q", templateID)
	}
	out := append([]string(nil), args...)
	if seed := Value(out, "--fingerprint"); seed != "" {
		stableSeed = seed
	}
	if stableSeed == "" {
		n, err := rand.Int(rand.Reader, big.NewInt(2147483646))
		if err != nil {
			return nil, err
		}
		stableSeed = strconv.FormatInt(n.Int64()+1, 10)
	}
	for _, kv := range [][2]string{{"--fingerprint", stableSeed}, {"--ant-identity-template", templateID}, {"--fingerprint-platform", chosen.Platform}, {"--fingerprint-platform-version", chosen.PlatformVersion}, {"--fingerprint-brand", "Chrome"}, {"--fingerprint-hardware-concurrency", strconv.Itoa(chosen.CPU)}, {"--window-size", chosen.WindowSize}, {"--ant-font-policy", "spoof"}} {
		out = Set(out, kv[0], kv[1])
	}
	return AlignVersion(out, kernelVersion)
}

func AlignVersion(args []string, kernelVersion string) ([]string, error) {
	i := FromArgs(args)
	major := Major(kernelVersion)
	if major <= 0 {
		return nil, fmt.Errorf("无法识别内核版本，不能生成自洽 UA；请先选择版本可识别的内核")
	}
	if i.Brand != "" && !strings.EqualFold(i.Brand, "Chrome") {
		return nil, fmt.Errorf("当前仅支持 Chrome 身份的版本对齐，请先选择 Chrome 品牌")
	}
	ua := ReducedUA(i.Platform, major)
	if ua == "" {
		return nil, fmt.Errorf("请先选择身份平台")
	}
	out := Set(args, "--user-agent", ua)
	return Set(out, "--fingerprint-brand-version", kernelVersion), nil
}

func ValidateArgs(args []string, kernelVersion string) []Issue {
	i := FromArgs(args)
	issues := Validate(i, kernelVersion)
	if i.FontPolicy == "spoof" {
		for _, v := range strings.Split(Value(args, "--disable-spoofing"), ",") {
			if strings.TrimSpace(v) == "font" {
				issues = append(issues, Issue{"fontPolicy", "warning", "已显式禁用字体伪装；请取消“禁用字体”才能保留内核字体伪装"})
			}
		}
	}
	if v := Value(args, "--fingerprint-hardware-concurrency"); v != "" {
		if n, e := strconv.Atoi(v); e != nil || n < 1 || n > 128 {
			issues = append(issues, Issue{"hardwareConcurrency", "error", "CPU 核心数须为 1–128 的整数"})
		}
	}
	if Value(args, "--ant-geolocation") != "" && i.Geo == nil {
		issues = append(issues, Issue{"geo", "error", "定位参数须为纬度,经度,精度"})
	}
	return issues
}
