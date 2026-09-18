package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const extensionSearchResultLimit = 12

func searchChromeWebStore(ctx context.Context, query string, client *http.Client) ([]ExtensionSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("请输入扩展名称")
	}
	if client == nil {
		client = &http.Client{Timeout: extensionDownloadTimeout}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, BuildChromeWebStoreSearchURL(query), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/148.0.0.0 Safari/537.36")
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("搜索 Chrome Web Store 失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("搜索 Chrome Web Store 失败: HTTP %d", response.StatusCode)
	}
	doc, err := html.Parse(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("解析 Chrome Web Store 搜索结果失败: %w", err)
	}
	return parseChromeWebStoreSearchResults(doc, extensionSearchResultLimit), nil
}

func parseChromeWebStoreSearchResults(root *html.Node, limit int) []ExtensionSearchResult {
	if root == nil || limit <= 0 {
		return nil
	}
	results := make([]ExtensionSearchResult, 0, limit)
	seen := make(map[string]struct{}, limit)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || len(results) >= limit {
			return
		}
		if node.Type == html.ElementNode {
			extensionID := NormalizeExtensionID(htmlNodeAttr(node, "data-item-id"))
			if extensionID != "" {
				if _, exists := seen[extensionID]; !exists {
					name := strings.TrimSpace(firstDescendantText(node, "h2"))
					if name == "" {
						name = extensionID
					}
					results = append(results, ExtensionSearchResult{
						ExtensionID: extensionID,
						Name:        name,
						StoreURL:    BuildChromeWebStoreURL(extensionID),
						IconURL:     firstDescendantAttr(node, "img", "src"),
					})
					seen[extensionID] = struct{}{}
				}
			}
		}
		for child := node.FirstChild; child != nil && len(results) < limit; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return results
}

func htmlNodeAttr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func firstDescendantText(root *html.Node, tag string) string {
	if root == nil {
		return ""
	}
	var result string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || result != "" {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
			result = strings.TrimSpace(nodeTextContent(node))
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return result
}

func firstDescendantAttr(root *html.Node, tag string, attrName string) string {
	if root == nil {
		return ""
	}
	var result string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil || result != "" {
			return
		}
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
			result = htmlNodeAttr(node, attrName)
			if result != "" {
				return
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return result
}

func nodeTextContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text := strings.TrimSpace(nodeTextContent(child))
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(text)
	}
	return strings.TrimSpace(builder.String())
}

func extractExtensionIDFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.ToLower(strings.TrimSpace(parts[i]))
		if extensionIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func downloadChromeExtensionCRX(ctx context.Context, extensionID string, client *http.Client) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: extensionDownloadTimeout}
	}
	downloadURL := BuildChromeExtensionDownloadURL(extensionID)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		data, err := downloadChromeExtensionCRXOnce(ctx, client, downloadURL)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if !isRetryableExtensionDownloadError(err) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 250 * time.Millisecond):
		}
	}
	return nil, formatExtensionDownloadError(lastErr)
}

func downloadChromeExtensionCRXOnce(ctx context.Context, client *http.Client, downloadURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36")
	request.Header.Set("Accept", "*/*")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载插件失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("下载插件失败: HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, extensionMaxPackageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("读取插件包失败: %w", err)
	}
	if len(data) > extensionMaxPackageBytes {
		return nil, fmt.Errorf("插件包超过限制")
	}
	return data, nil
}

func isRetryableExtensionDownloadError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "eof") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "connection refused") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "temporarily unavailable")
}

func formatExtensionDownloadError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "eof") {
		return fmt.Errorf("下载插件失败: 连接在下载过程中提前关闭（EOF），已重试 3 次仍失败；请换一个下载代理节点或稍后重试")
	}
	if strings.Contains(message, "connectex") || strings.Contains(message, "dial tcp") || strings.Contains(message, "i/o timeout") {
		return fmt.Errorf("下载插件失败: 无法连接 Chrome 插件下载服务，请确认网络或下载代理可访问 clients2.google.com: %w", err)
	}
	return err
}

func normalizeExtensionArchiveData(data []byte) ([]byte, error) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return data, nil
	}
	zipOffset := bytes.Index(data, []byte("PK\x03\x04"))
	if zipOffset < 0 {
		return nil, fmt.Errorf("插件包不是有效的 CRX/ZIP 文件")
	}
	return data[zipOffset:], nil
}

func readExtensionManifestFromZip(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("打开插件包失败: %w", err)
	}
	for _, file := range reader.File {
		if normalizeZipEntryPath(file.Name) == "manifest.json" {
			return readZipFile(file, 2<<20)
		}
	}
	return nil, fmt.Errorf("插件包缺少 manifest.json")
}

func parseExtensionManifest(data []byte) (extensionManifest, error) {
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return manifest, fmt.Errorf("manifest.json 缺少 version")
	}
	return manifest, nil
}

func readExtensionLocaleMessagesFromZip(data []byte, manifest extensionManifest) map[string]string {
	locale := resolveExtensionLocale(manifest)
	if locale == "" {
		return nil
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(normalizeZipEntryPath(file.Name), "_locales/"+locale+"/messages.json") {
			content, err := readZipFile(file, 1<<20)
			if err != nil {
				return nil
			}
			return parseExtensionLocaleMessages(content)
		}
	}
	return nil
}

func readExtensionLocaleMessagesFromDir(sourceDir string, manifest extensionManifest) map[string]string {
	locale := resolveExtensionLocale(manifest)
	if locale == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(sourceDir, "_locales", locale, "messages.json"))
	if err != nil {
		return nil
	}
	return parseExtensionLocaleMessages(data)
}

func parseExtensionLocaleMessages(data []byte) map[string]string {
	var raw map[string]struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	messages := make(map[string]string, len(raw))
	for key, value := range raw {
		if key = strings.TrimSpace(key); key != "" {
			messages[key] = strings.TrimSpace(value.Message)
		}
	}
	return messages
}

func resolveExtensionLocale(manifest extensionManifest) string {
	locale := strings.TrimSpace(manifest.DefaultLocale)
	locale = strings.Trim(locale, "/\\. ")
	if locale == "" || strings.Contains(locale, "/") || strings.Contains(locale, "\\") {
		return ""
	}
	return locale
}

func resolveExtensionDescription(manifest extensionManifest, messages map[string]string) string {
	return resolveExtensionMessage(strings.TrimSpace(manifest.Description), messages)
}

func resolveExtensionMessage(value string, messages map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "__MSG_") || !strings.HasSuffix(trimmed, "__") {
		return trimmed
	}
	key := strings.TrimSuffix(strings.TrimPrefix(trimmed, "__MSG_"), "__")
	if message := strings.TrimSpace(messages[key]); message != "" {
		return message
	}
	return trimmed
}

func readExtensionIconDataURLFromZip(data []byte, manifest extensionManifest) string {
	iconPath := resolveExtensionIconPath(manifest)
	if iconPath == "" {
		return ""
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(normalizeZipEntryPath(file.Name), iconPath) {
			content, err := readZipFile(file, 1<<20)
			if err != nil {
				return ""
			}
			return extensionIconDataURL(iconPath, content)
		}
	}
	return ""
}

func readExtensionIconDataURLFromDir(sourceDir string, manifest extensionManifest) string {
	iconPath := resolveExtensionIconPath(manifest)
	if iconPath == "" {
		return ""
	}
	fullPath := filepath.Join(sourceDir, filepath.FromSlash(iconPath))
	content, err := os.ReadFile(fullPath)
	if err != nil || len(content) > 1<<20 {
		return ""
	}
	return extensionIconDataURL(iconPath, content)
}

func resolveExtensionIconPath(manifest extensionManifest) string {
	for _, candidate := range []map[string]any{manifest.Action, manifest.BrowserAction} {
		if path := mapStringValue(candidate, "default_icon"); path != "" {
			return normalizeExtensionAssetPath(path)
		}
	}
	bestSize := -1
	bestPath := ""
	for size, path := range manifest.Icons {
		if normalizedPath := normalizeExtensionAssetPath(path); normalizedPath != "" {
			parsedSize := parseExtensionIconSize(size)
			if parsedSize > bestSize {
				bestSize = parsedSize
				bestPath = normalizedPath
			}
		}
	}
	return bestPath
}

func mapStringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return value
	}
	if nested, ok := values[key].(map[string]any); ok {
		bestSize := -1
		bestPath := ""
		for size, rawPath := range nested {
			path, ok := rawPath.(string)
			if !ok {
				continue
			}
			parsedSize := parseExtensionIconSize(size)
			if parsedSize > bestSize {
				bestSize = parsedSize
				bestPath = path
			}
		}
		return bestPath
	}
	return ""
}

func normalizeExtensionAssetPath(value string) string {
	path := strings.TrimSpace(filepath.ToSlash(value))
	path = strings.TrimLeft(path, "/")
	if path == "" || strings.Contains(path, "..") || filepath.IsAbs(path) {
		return ""
	}
	return path
}

func parseExtensionIconSize(value string) int {
	var size int
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &size)
	return size
}

func extensionIconDataURL(path string, data []byte) string {
	if len(data) == 0 || len(data) > 1<<20 {
		return ""
	}
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(mimeType, "image/") {
		return ""
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func replaceExtensionDirFromZip(data []byte, installDir string) error {
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		return fmt.Errorf("创建插件父目录失败: %w", err)
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(installDir), "."+filepath.Base(installDir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建插件目录失败: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("打开插件包失败: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		relativePath := normalizeZipEntryPath(file.Name)
		if relativePath == "" {
			continue
		}
		targetPath := filepath.Join(tmpDir, filepath.FromSlash(relativePath))
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(tmpDir)+string(os.PathSeparator)) {
			return fmt.Errorf("插件包包含非法路径: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("创建插件文件目录失败: %w", err)
		}
		content, err := readZipFile(file, extensionMaxPackageBytes)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return fmt.Errorf("写入插件文件失败: %w", err)
		}
	}
	if err := commitExtensionDirectory(tmpDir, installDir); err != nil {
		return err
	}
	success = true
	return nil
}

func copyExtensionDirectory(sourceDir string, installDir string) error {
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		return fmt.Errorf("创建插件父目录失败: %w", err)
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(installDir), "."+filepath.Base(installDir)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建插件目录失败: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	sourceClean, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	if err := filepath.WalkDir(sourceClean, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("插件目录包含符号链接: %s", path)
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > extensionMaxPackageBytes {
			return fmt.Errorf("插件文件过大: %s", path)
		}
		relativePath, err := filepath.Rel(sourceClean, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(tmpDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	}); err != nil {
		return fmt.Errorf("复制插件目录失败: %w", err)
	}
	if err := commitExtensionDirectory(tmpDir, installDir); err != nil {
		return err
	}
	success = true
	return nil
}

type extensionInstallFileTransaction struct {
	installDir       string
	installBackupDir string
	hadInstallDir    bool
	packagePath      string
	packageBackup    string
	hadPackage       bool
}

func beginExtensionInstallFileTransaction(installDir string) (*extensionInstallFileTransaction, error) {
	installDir = filepath.Clean(strings.TrimSpace(installDir))
	if installDir == "" || installDir == "." {
		return nil, fmt.Errorf("插件安装目录不能为空")
	}
	tx := &extensionInstallFileTransaction{installDir: installDir}
	if _, err := os.Lstat(installDir); err == nil {
		tx.installBackupDir = fmt.Sprintf("%s.rollback-%d", installDir, time.Now().UnixNano())
		if err := os.Rename(installDir, tx.installBackupDir); err != nil {
			return nil, fmt.Errorf("暂存旧插件目录失败: %w", err)
		}
		tx.hadInstallDir = true
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("检查旧插件目录失败: %w", err)
	}
	return tx, nil
}

func (tx *extensionInstallFileTransaction) stagePackage(packagePath string) error {
	if tx == nil {
		return fmt.Errorf("插件安装事务未初始化")
	}
	packagePath = filepath.Clean(strings.TrimSpace(packagePath))
	if packagePath == "" || packagePath == "." {
		return fmt.Errorf("插件包路径不能为空")
	}
	tx.packagePath = packagePath
	if _, err := os.Lstat(packagePath); err == nil {
		tx.packageBackup = fmt.Sprintf("%s.rollback-%d", packagePath, time.Now().UnixNano())
		if err := os.Rename(packagePath, tx.packageBackup); err != nil {
			return fmt.Errorf("暂存旧插件包失败: %w", err)
		}
		tx.hadPackage = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查旧插件包失败: %w", err)
	}
	return nil
}

func (tx *extensionInstallFileTransaction) rollback() error {
	if tx == nil {
		return nil
	}
	rollbackErrors := make([]error, 0, 4)
	if strings.TrimSpace(tx.packagePath) != "" {
		if err := os.Remove(tx.packagePath); err != nil && !os.IsNotExist(err) {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("删除新插件包失败: %w", err))
		}
		if tx.hadPackage {
			if err := os.Rename(tx.packageBackup, tx.packagePath); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复旧插件包失败: %w", err))
			}
		}
	}
	if strings.TrimSpace(tx.installDir) != "" {
		if err := os.RemoveAll(tx.installDir); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("删除新插件目录失败: %w", err))
		}
		if tx.hadInstallDir {
			if err := os.Rename(tx.installBackupDir, tx.installDir); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("恢复旧插件目录失败: %w", err))
			}
		}
	}
	return errors.Join(rollbackErrors...)
}

func (tx *extensionInstallFileTransaction) commit() {
	if tx == nil {
		return
	}
	if tx.hadPackage && strings.TrimSpace(tx.packageBackup) != "" {
		_ = os.Remove(tx.packageBackup)
	}
	if tx.hadInstallDir && strings.TrimSpace(tx.installBackupDir) != "" {
		_ = os.RemoveAll(tx.installBackupDir)
	}
}

func rollbackExtensionInstallFiles(tx *extensionInstallFileTransaction, cause error) error {
	if tx == nil {
		return cause
	}
	return errors.Join(cause, tx.rollback())
}

func commitExtensionDirectory(stagedDir string, installDir string) error {
	stagedDir = filepath.Clean(stagedDir)
	installDir = filepath.Clean(installDir)
	if stagedDir == installDir {
		return fmt.Errorf("插件暂存目录不能与安装目录相同")
	}
	if info, err := os.Stat(stagedDir); err != nil {
		return fmt.Errorf("检查插件暂存目录失败: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("插件暂存路径不是目录: %s", stagedDir)
	}

	backupDir := fmt.Sprintf("%s.rollback-%d", installDir, time.Now().UnixNano())
	hadOriginal := false
	if _, err := os.Stat(installDir); err == nil {
		hadOriginal = true
		if err := os.Rename(installDir, backupDir); err != nil {
			return fmt.Errorf("暂存旧插件目录失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查旧插件目录失败: %w", err)
	}

	if err := os.Rename(stagedDir, installDir); err != nil {
		commitErr := fmt.Errorf("安装插件失败: %w", err)
		if hadOriginal {
			if restoreErr := os.Rename(backupDir, installDir); restoreErr != nil {
				return fmt.Errorf("%w；恢复旧插件目录失败: %v", commitErr, restoreErr)
			}
		}
		return commitErr
	}

	if hadOriginal {
		// The new directory is already committed. Cleanup failure must not turn
		// a successful update into an application-level failure.
		_ = os.RemoveAll(backupDir)
	}
	return nil
}

func normalizeZipEntryPath(value string) string {
	path := strings.TrimSpace(filepath.ToSlash(value))
	path = strings.TrimLeft(path, "/")
	if path == "" || strings.Contains(path, "..") || filepath.IsAbs(path) {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) > 1 && parts[0] != "" && parts[1] == "manifest.json" {
		return strings.Join(parts[1:], "/")
	}
	return path
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("读取插件文件失败: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("读取插件文件失败: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("插件文件过大: %s", file.Name)
	}
	return data, nil
}

func extensionIDFromManifest(manifestData []byte) string {
	sum := sha256.Sum256(manifestData)
	hexValue := hex.EncodeToString(sum[:16])
	var builder strings.Builder
	for _, char := range hexValue {
		if char >= '0' && char <= '9' {
			builder.WriteByte(byte('a' + char - '0'))
			continue
		}
		builder.WriteByte(byte('k' + char - 'a'))
	}
	return builder.String()
}

func resolveExtensionName(manifest extensionManifest, fallback string) string {
	for _, value := range []string{manifest.Name, manifest.ShortName, fallback} {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return "Chrome 插件"
}
