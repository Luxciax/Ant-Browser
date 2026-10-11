package backend

import (
	"strconv"
	"strings"
)

type BrowserFingerprintWorkerInfo struct {
	Language            string   `json:"language"`
	Languages           []string `json:"languages"`
	Timezone            string   `json:"timezone"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	Platform            string   `json:"platform"`
	UserAgent           string   `json:"userAgent"`
	Error               string   `json:"error,omitempty"`
}

func assessFingerprintRuntime(r BrowserFingerprintRuntimeInfo, e BrowserFingerprintExpectedInfo) (string, []string) {
	issues := []string{}
	check := func(label, expected, actual string) {
		if expected != "" && expected != actual {
			issues = append(issues, label+"：配置与运行值不一致")
		}
	}
	check("语言", e.Language, r.Language)
	check("时区", e.Timezone, r.Timezone)
	check("CPU", e.HardwareConcurrency, strconv.Itoa(r.HardwareConcurrency))
	check("WebGL Vendor", e.WebGLVendor, r.WebGLVendor)
	check("WebGL Renderer", e.WebGLRenderer, r.WebGLRenderer)
	switch strings.ToLower(e.Platform) {
	case "windows":
		check("平台", "Win32", r.Platform)
	case "linux":
		check("平台", "Linux x86_64", r.Platform)
	case "mac", "macos":
		check("平台", "MacIntel", r.Platform)
	}
	if r.Webdriver {
		issues = append(issues, "navigator.webdriver 为 true")
	}
	workerOK := r.Worker != nil && r.Worker.Error == "" && r.Worker.Language != "" && r.Worker.HardwareConcurrency > 0
	if workerOK {
		w := r.Worker
		check("Worker 语言", r.Language, w.Language)
		check("Worker 语言列表", strings.Join(r.Languages, ","), strings.Join(w.Languages, ","))
		check("Worker 时区", r.Timezone, w.Timezone)
		check("Worker CPU", strconv.Itoa(r.HardwareConcurrency), strconv.Itoa(w.HardwareConcurrency))
		check("Worker 平台", r.Platform, w.Platform)
		check("Worker UA", r.UserAgent, w.UserAgent)
	}
	if len(issues) > 0 {
		return "mismatch", issues
	}
	if !workerOK {
		return "unverified", []string{"Worker 未返回有效结果（可能受页面 CSP 限制），不能判定一致"}
	}
	return "passed_basic", issues
}
