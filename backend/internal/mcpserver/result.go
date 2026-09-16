package mcpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ant-chrome/backend/internal/launchcode"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: describeError(err)}}}
}

func describeError(err error) string {
	if err == nil {
		return "unknown error"
	}
	var svcErr *launchcode.ServiceError
	if !errors.As(err, &svcErr) {
		return err.Error()
	}
	switch {
	case svcErr.NotFound():
		return fmt.Sprintf("%s。请先用 ant_instance_list 确认目标是否存在。", svcErr.Message)
	case svcErr.Ambiguous():
		if strings.Contains(svcErr.Message, "matched") || strings.Contains(svcErr.Message, "ambiguous") {
			return fmt.Sprintf("%s。请补充更精确的条件（code / profileId），或显式设置 matchMode=first。", svcErr.Message)
		}
		return svcErr.Message
	case svcErr.Unavailable():
		return fmt.Sprintf("%s。该能力在当前运行环境不可用。", svcErr.Message)
	case svcErr.Status == http.StatusBadRequest:
		return fmt.Sprintf("参数无效：%s", svcErr.Message)
	default:
		return svcErr.Message
	}
}

func newInputError(message string) error {
	return &launchcode.ServiceError{Status: http.StatusBadRequest, Message: message}
}

func errorFromRun(run ScriptRun) error {
	message := strings.TrimSpace(run.Error)
	if message == "" {
		message = strings.TrimSpace(run.Summary)
	}
	if message == "" {
		message = "脚本执行失败但未返回错误信息"
	}
	return &launchcode.ServiceError{Status: http.StatusInternalServerError, Message: "脚本执行失败：" + message}
}

func readOnly(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true}
}

func mutating(title string) *mcp.ToolAnnotations {
	no := false
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &no}
}

func destructive(title string) *mcp.ToolAnnotations {
	yes := true
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &yes}
}
