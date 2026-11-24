package backend

import (
	"strconv"
	"strings"
)

type BrowserFingerprintWorkerInfo struct {
	Applicable          bool     `json:"applicable"`
	Executed            bool     `json:"executed"`
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
	verificationGaps := []string{}
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
	checkContext := func(label string, ctx *BrowserFingerprintWorkerInfo) {
		if ctx == nil {
			return
		}
		legacyObservation := ctx.Language != "" || len(ctx.Languages) > 0 || ctx.Timezone != "" ||
			ctx.HardwareConcurrency != 0 || ctx.Platform != "" || ctx.UserAgent != ""
		if !ctx.Applicable && !legacyObservation {
			return
		}
		if (!ctx.Executed && !legacyObservation) || ctx.Error != "" {
			verificationGaps = append(verificationGaps, label+" 未执行有效探测")
			return
		}
		check(label+" 语言", r.Language, ctx.Language)
		check(label+" 语言列表", strings.Join(r.Languages, ","), strings.Join(ctx.Languages, ","))
		check(label+" 时区", r.Timezone, ctx.Timezone)
		check(label+" CPU", strconv.Itoa(r.HardwareConcurrency), strconv.Itoa(ctx.HardwareConcurrency))
		check(label+" 平台", r.Platform, ctx.Platform)
		check(label+" UA", r.UserAgent, ctx.UserAgent)
	}
	if r.Worker == nil {
		verificationGaps = append(verificationGaps, "DedicatedWorker 未提供探测结果")
	} else {
		checkContext("DedicatedWorker", r.Worker)
	}
	checkContext("Iframe", r.Iframe)
	checkContext("SharedWorker", r.SharedWorker)
	if r.ServiceWorker.Applicable && !r.ServiceWorker.Executed {
		verificationGaps = append(verificationGaps, "ServiceWorker 适用但没有受控探测端点，未执行跨上下文断言")
	}
	if len(issues) > 0 {
		return "mismatch", issues
	}
	if len(verificationGaps) > 0 {
		return "unverified", verificationGaps
	}
	return "passed_basic", issues
}
