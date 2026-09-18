package backend

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"ant-chrome/backend/internal/automation"
)

func cloneAutomationGitRepository(repoURL string, ref string) (string, func(), error) {
	normalizedRepoURL, err := validateAutomationGitRepositoryURL(repoURL)
	if err != nil {
		return "", nil, err
	}
	normalizedRef, err := validateAutomationGitRef(ref)
	if err != nil {
		return "", nil, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, fmt.Errorf("未找到 git，可先安装 git 后再导入仓库脚本")
	}

	tempDir, err := os.MkdirTemp("", "ant-automation-git-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建 Git 临时目录失败: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	if normalizedRef == "" {
		if err := runGitCommand("", "clone", "--depth", "1", "--", normalizedRepoURL, tempDir); err != nil {
			cleanup()
			return "", nil, err
		}
		return tempDir, cleanup, nil
	}

	if err := runGitCommand("", "clone", "--depth", "1", "--branch", normalizedRef, "--single-branch", "--", normalizedRepoURL, tempDir); err == nil {
		return tempDir, cleanup, nil
	}

	_ = os.RemoveAll(tempDir)
	tempDir, err = os.MkdirTemp("", "ant-automation-git-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建 Git 临时目录失败: %w", err)
	}
	cleanup = func() {
		_ = os.RemoveAll(tempDir)
	}

	if err := runGitCommand("", "clone", "--", normalizedRepoURL, tempDir); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := runGitCommand(tempDir, "checkout", "--detach", normalizedRef); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("切换 Git 引用失败: %w", err)
	}
	return tempDir, cleanup, nil
}

func validateAutomationGitRepositoryURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Git 仓库地址不能为空")
	}
	parsed, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return "", fmt.Errorf("Git 仓库地址格式无效")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("Git 仓库仅支持 https:// 地址")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("Git 仓库地址不能包含内嵌凭据")
	}
	return parsed.String(), nil
}

func validateAutomationGitRef(raw string) (string, error) {
	ref := strings.TrimSpace(raw)
	if ref == "" {
		return "", nil
	}
	if strings.HasPrefix(ref, "-") || strings.HasPrefix(ref, ".") || strings.HasSuffix(ref, ".") ||
		strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") ||
		strings.HasSuffix(ref, "/") || strings.HasSuffix(strings.ToLower(ref), ".lock") {
		return "", fmt.Errorf("Git 引用格式无效")
	}
	for _, r := range ref {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("~^:?*[\\", r) {
			return "", fmt.Errorf("Git 引用格式无效")
		}
	}
	return ref, nil
}

func runGitCommand(workdir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if strings.TrimSpace(workdir) != "" {
		cmd.Dir = workdir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("git %s 失败: %s", strings.Join(args, " "), message)
	}
	return nil
}

func (a *App) loadAutomationGitBundle(repoURL string, ref string, scriptPath string) (automation.ImportedBundle, error) {
	normalizedRepoURL := strings.TrimSpace(repoURL)
	if normalizedRepoURL == "" {
		return automation.ImportedBundle{}, fmt.Errorf("Git 仓库地址不能为空")
	}

	normalizedRef := strings.TrimSpace(ref)
	normalizedScriptPath := strings.TrimSpace(scriptPath)

	repoDir, cleanup, err := cloneAutomationGitRepository(normalizedRepoURL, normalizedRef)
	if err != nil {
		return automation.ImportedBundle{}, err
	}
	defer cleanup()

	bundle, err := automation.ImportBundleFromDirectoryWithOptions(repoDir, normalizedScriptPath, buildAutomationGitImportLabel(normalizedRepoURL, normalizedRef, normalizedScriptPath), a.automationScriptImportOptions())
	if err != nil {
		return automation.ImportedBundle{}, err
	}
	return bundle, nil
}
