package cli

import (
	"context"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/claudecode"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/codex"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/mcode"
	opencodeagent "github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/opencode"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/pi"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func registerAgentKinds(registry *agent.Registry, agentCLIs agentCLIDiscovery, serverURL string) {
	registerProduct := func(info proto.SupportedAgentKind, factory agent.Factory) {
		if agentCLIs.permits(info.Kind) {
			registerProductAgentKind(registry, info, factory)
		}
	}

	registerProduct(agentCLIs.ClaudeCode, withSkillUploadServer(withCapabilityDownloads(claudecode.Factory, serverURL), serverURL))
	registerProduct(agentCLIs.OpenCode, withSkillUploadServer(withCapabilityDownloads(opencodeagent.Factory, serverURL), serverURL))
	registerProduct(agentCLIs.Codex, withSkillUploadServer(withCapabilityDownloads(codex.Factory, serverURL), serverURL))
	if agentCLIs.Codex.Available {
		registry.RegisterExecutor("codex", withExecutorCapabilities(codex.NewExecutorFactory(), serverURL))
	}
	if agentCLIs.Codex.Available && agentCLIs.Codex.Capabilities.LocalEnvironment {
		registry.RegisterPreparation("codex", true, func(ctx context.Context, req proto.PromptRequestPayload) (agent.Prepared, error) {
			prepared, err := codex.Prepare(ctx, req)
			if prepared == nil {
				return nil, err
			}
			return prepared, err
		})
	}
	registerProduct(agentCLIs.Pi, withSkillUploadServer(withCapabilityDownloads(pi.Factory, serverURL), serverURL))
	if agentCLIs.MCodeWorkspace != nil {
		registry.RegisterKind(agentCLIs.MCode, mcode.Factory)
		registry.RegisterPreparation("mcode", true, mcode.NewPreparationFactory(*agentCLIs.MCodeWorkspace))
	} else {
		registerProduct(agentCLIs.MCode, withSkillUploadServer(withCapabilityDownloads(mcode.Factory, serverURL), serverURL))
	}
	if agentCLIs.MCode.Available {
		registry.RegisterExecutor("mcode", withExecutorCapabilities(mcode.NewExecutorFactory(agentCLIs.MCodeWorkspace), serverURL))
	}
	registerClaudeSDK(registry, agentCLIs.ClaudeSDK)
}

func registerProductAgentKind(registry *agent.Registry, info proto.SupportedAgentKind, factory agent.Factory) {
	info.Capabilities.WorkspaceAuthoring = true
	registry.RegisterKind(info, factory)
}
