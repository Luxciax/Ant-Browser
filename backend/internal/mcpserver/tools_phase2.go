package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type phase2InstanceProvider interface {
	EnvironmentContract(profileID string) (map[string]interface{}, error)
	LaunchPlanState(profileID string) (map[string]interface{}, error)
	LaunchReceipt(profileID string) (map[string]interface{}, error)
	KernelCapabilities(profileID string) (map[string]interface{}, error)
	RuntimeEvidence(profileID string) (map[string]interface{}, error)
	VerifyRuntime(profileID string) (map[string]interface{}, error)
}

type phase2ReadInput struct {
	Selector Selector `json:"selector"`
}

type phase2Output struct {
	Data map[string]interface{} `json:"data"`
}

func resolvePhase2Profile(p InstanceProvider, selector Selector) (string, error) {
	profile, err := p.FindProfile(selector.toLaunchSelector())
	if err != nil {
		return "", err
	}
	if profile == nil || profile.ProfileId == "" {
		return "", fmt.Errorf("profile not found")
	}
	return profile.ProfileId, nil
}

func registerPhase2Tools(srv *mcp.Server, p InstanceProvider) {
	inspector, ok := p.(phase2InstanceProvider)
	if !ok {
		return
	}
	addRead := func(name, description, title string, read func(string) (map[string]interface{}, error)) {
		mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description, Annotations: readOnly(title)}, func(_ context.Context, _ *mcp.CallToolRequest, in phase2ReadInput) (*mcp.CallToolResult, phase2Output, error) {
			profileID, err := resolvePhase2Profile(p, in.Selector)
			if err != nil {
				return toolError(err), phase2Output{}, nil
			}
			data, err := read(profileID)
			if err != nil {
				return toolError(err), phase2Output{}, nil
			}
			return nil, phase2Output{Data: data}, nil
		})
	}
	addRead("ant_instance_environment", "读取实例的 v2 环境契约，不返回原始代理或启动参数。", "读取环境契约", inspector.EnvironmentContract)
	addRead("ant_instance_launch_state", "读取配置当前是 configured、effective 还是 pending-next-launch。", "读取启动状态", inspector.LaunchPlanState)
	addRead("ant_instance_launch_receipt", "读取最近一次统一 LaunchPlan 的运行收据；不会触发启动或验证。", "读取启动收据", inspector.LaunchReceipt)
	addRead("ant_instance_capabilities", "读取实际内核能力阶段；未有证据的能力不会标记 verified。", "读取内核能力", inspector.KernelCapabilities)
	addRead("ant_runtime_evidence", "读取最近一次显式运行验证留下的证据；不会自动执行验证。", "读取运行证据", inspector.RuntimeEvidence)

	mcp.AddTool(srv, &mcp.Tool{Name: "ant_runtime_verify", Description: "显式执行一次基础运行验证并保存证据；普通状态查询不会调用它。", Annotations: mutating("验证运行配置")}, func(_ context.Context, _ *mcp.CallToolRequest, in phase2ReadInput) (*mcp.CallToolResult, phase2Output, error) {
		profileID, err := resolvePhase2Profile(p, in.Selector)
		if err != nil {
			return toolError(err), phase2Output{}, nil
		}
		data, err := inspector.VerifyRuntime(profileID)
		if err != nil {
			return toolError(err), phase2Output{}, nil
		}
		return nil, phase2Output{Data: data}, nil
	})
}

