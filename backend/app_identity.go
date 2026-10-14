package backend

import (
	"ant-chrome/backend/internal/identity"
	"fmt"
	"sort"
	"strings"
)

type BrowserIdentityReport struct {
	Identity          identity.Identity `json:"identity"`
	Hash              string            `json:"hash"`
	KernelVersion     string            `json:"kernelVersion"`
	Issues            []identity.Issue  `json:"issues"`
	DuplicateProfiles []string          `json:"duplicateProfiles"`
}

type BrowserIdentityInput struct {
	ProfileID   string        `json:"profileId"`
	CoreID      string        `json:"coreId"`
	Args        []string      `json:"args"`
	TemplateID  string        `json:"templateId"`
	AlignKernel bool          `json:"alignKernel"`
	Geo         *identity.Geo `json:"geo,omitempty"`
	ClearGeo    bool          `json:"clearGeo"`
}

type BrowserIdentityResult struct {
	Args   []string              `json:"args"`
	Report BrowserIdentityReport `json:"report"`
}

func (a *App) BrowserIdentityTemplates() []identity.Template { return identity.Templates() }

// Preview only. Normal profile Save persists returned args through the existing
// DAO; canceling the editor never changes a running environment.
func (a *App) BrowserIdentityGenerate(input BrowserIdentityInput) (BrowserIdentityResult, error) {
	if len(input.Args) > 256 {
		return BrowserIdentityResult{}, fmt.Errorf("指纹参数数量超过限制")
	}
	core, ok := a.resolveBrowserCoreForFingerprintReport(input.CoreID)
	version := ""
	if ok {
		version = a.browserMgr.GetChromeVersion(core.CorePath)
	}
	args := append([]string(nil), input.Args...)
	var err error
	if input.TemplateID != "" {
		seed := ""
		if input.ProfileID != "" {
			seed = fmt.Sprint(browserFingerprintSeedForProfileID(input.ProfileID))
		}
		args, err = identity.ApplyTemplate(args, input.TemplateID, seed, version)
	} else if input.AlignKernel {
		args, err = identity.AlignVersion(args, version)
	}
	if err != nil {
		return BrowserIdentityResult{}, err
	}
	if input.ClearGeo {
		args = identity.Set(args, "--ant-geolocation", "")
	}
	if input.Geo != nil {
		if !identity.ValidGeo(*input.Geo) {
			return BrowserIdentityResult{}, fmt.Errorf("定位坐标或精度无效")
		}
		args = identity.Set(args, "--ant-geolocation", fmt.Sprintf("%g,%g,%g", input.Geo.Latitude, input.Geo.Longitude, input.Geo.Accuracy))
	}
	args = normalizeBrowserLanguageArgs(args)
	report := a.browserIdentityReport(input.ProfileID, args, version)
	for _, issue := range report.Issues {
		if issue.Level == "error" {
			return BrowserIdentityResult{}, fmt.Errorf("%s", issue.Message)
		}
	}
	return BrowserIdentityResult{args, report}, nil
}

func (a *App) browserIdentityReport(profileID string, args []string, version string) BrowserIdentityReport {
	id := identity.FromArgs(args)
	if id.Seed == "" && profileID != "" {
		id.Seed = fmt.Sprint(browserFingerprintSeedForProfileID(profileID))
	}
	report := BrowserIdentityReport{Identity: id, Hash: id.Hash(), KernelVersion: version, Issues: identity.ValidateArgs(args, version), DuplicateProfiles: []string{}}
	if a != nil && a.browserMgr != nil && id.Seed != "" {
		a.browserMgr.Mutex.Lock()
		for _, p := range a.browserMgr.Profiles {
			if p == nil || p.ProfileId == profileID || p.DeletedAt != "" {
				continue
			}
			other := identity.FromArgs(p.FingerprintArgs)
			if other.Seed == "" {
				other.Seed = fmt.Sprint(browserFingerprintSeedForProfileID(p.ProfileId))
			}
			if other.Seed == id.Seed {
				report.DuplicateProfiles = append(report.DuplicateProfiles, p.ProfileId)
			}
		}
		a.browserMgr.Mutex.Unlock()
		sort.Strings(report.DuplicateProfiles)
		if len(report.DuplicateProfiles) > 0 {
			report.Issues = append(report.Issues, identity.Issue{Field: "seed", Level: "warning", Message: "其他环境使用相同种子；不要把同一设备画像用于需要隔离的账号"})
		}
	}
	return report
}

func fontPassthroughForArgs(args []string) bool {
	// Absence preserves pre-upgrade behavior. New templates opt into native font
	// spoofing explicitly; that policy can expose missing cross-platform fonts.
	return !strings.EqualFold(identity.Value(args, "--ant-font-policy"), "spoof")
}
