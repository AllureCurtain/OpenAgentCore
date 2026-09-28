package execution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"github.com/google/uuid"
)

// runtimeSetupOperation is the packaged initializer's confidential stdin contract.
// Public templates and native harness configuration never cross this boundary.
type runtimeSetupOperation struct {
	Capabilities *proto.CapabilitiesPreparePayload `json:"-"`
	Archive      []byte                            `json:"-"`
	Version      int                               `json:"version"`
	Action       string                            `json:"action"`
	Network      string                            `json:"network,omitempty"`
	Env          map[string]string                 `json:"env"`
	Packages     []string                          `json:"packages,omitempty"`
	Command      string                            `json:"command,omitempty"`
	CWD          string                            `json:"cwd,omitempty"`
	// Index is the setup command position, used only for the public failure label.
	Index int `json:"-"`
}

// runtimeStepFailure is a confirmed failed initialization receipt. It holds only
// the Runtime-reported exit status (0 when absent), never command or output.
type runtimeStepFailure struct{ exitCode int }

func (*runtimeStepFailure) Error() string { return "environment initialization operation failed" }

// provisioningFailure labels a confirmed failed setup operation for the Store,
// which composes the public reason. Other actions keep the generic reason.
func (operation runtimeSetupOperation) provisioningFailure(exitCode int) store.ProvisioningFailure {
	switch operation.Action {
	case "setup", "python", "npm", "system", "skill":
		return store.ProvisioningFailure{Step: operation.Action, Index: operation.Index, ExitCode: exitCode}
	}
	return store.ProvisioningFailure{}
}

func setupOperations(setup store.EnvironmentSetup) []runtimeSetupOperation {
	if setup.Empty() {
		return nil
	}
	env := setup.Env
	if env == nil {
		env = map[string]string{}
	}
	result := []runtimeSetupOperation{{Version: 1, Action: "configure", Env: env}}
	for i := range setup.Skills {
		metadata := setup.Skills[i].InstallationMetadata()
		result = append(result, runtimeSetupOperation{Action: "skill", Capabilities: &proto.CapabilitiesPreparePayload{Action: "skill", Skill: &metadata}, Archive: setup.Skills[i].Archive})
	}
	for i, plugin := range setup.Plugins {
		result = append(result, runtimeSetupOperation{Capabilities: &proto.CapabilitiesPreparePayload{Action: "plugin", Slot: i, Plugin: &plugin.Metadata}, Archive: plugin.Archive})
	}
	// The public network policy applies after setup completes. Provisioning uses
	// the isolated initializer's network; adapters enforce the runtime policy.
	const network = "enabled"
	if len(setup.Packages.System) > 0 {
		result = append(result, runtimeSetupOperation{Version: 1, Action: "system", Network: network, Packages: setup.Packages.System})
	}
	if len(setup.Packages.NPM) > 0 {
		result = append(result, runtimeSetupOperation{Version: 1, Action: "npm", Network: network, Packages: setup.Packages.NPM})
	}
	if len(setup.Packages.Python) > 0 {
		result = append(result, runtimeSetupOperation{Version: 1, Action: "python", Network: network, Packages: setup.Packages.Python})
	}
	for i, command := range setup.Commands {
		cwd := command.CWD
		if cwd == "" {
			cwd = "/workspace"
		}
		result = append(result, runtimeSetupOperation{Version: 1, Action: "setup", Network: network, Command: command.Command, CWD: cwd, Index: i})
	}
	if len(setup.Skills)+len(setup.Plugins)+len(setup.CapabilityDirectories) > 0 {
		sources := agentcapabilities.Input{Plugins: setup.PluginMetadata(), Directories: setup.CapabilityDirectories}
		for _, skill := range setup.Skills {
			sources.Skills = append(sources.Skills, skill.InstallationMetadata())
		}
		result = append(result, runtimeSetupOperation{Capabilities: &proto.CapabilitiesPreparePayload{Action: "finalize", Sources: &sources}})
	}
	return result
}

func runRuntimeSetup(ctx context.Context, provider sandbox.Provider, reference sandbox.Reference, operation runtimeSetupOperation) error {
	if provider == nil || operation.Capabilities != nil {
		return sandbox.ErrInvalid
	}
	var payload any = operation
	args := []string{"/usr/bin/python3", "-I", "-S", "/usr/local/bin/oac-runtime-initialize"}
	input, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	result, err := provider.RunCommand(ctx, reference, sandbox.Command{Directory: "/", Args: args, Stdin: input})
	if err != nil {
		return err
	}
	var receipt struct {
		Version  int    `json:"version"`
		Outcome  string `json:"outcome"`
		ExitCode *int   `json:"exit_code"`
	}
	valid := result.Stderr == "" && json.Unmarshal([]byte(result.Stdout), &receipt) == nil && receipt.Version == 1
	if valid && result.ExitCode == 0 && receipt.Outcome == "completed" {
		return nil
	}
	// The initializer confirms a failed step with status 1 and, for a sandboxed
	// step, that step's exit status only. Images without exit_code stay generic.
	if valid && result.ExitCode == 1 && receipt.Outcome == "failed" && operation.Capabilities == nil {
		failure := &runtimeStepFailure{}
		if receipt.ExitCode != nil && *receipt.ExitCode > 0 && *receipt.ExitCode < 256 {
			failure.exitCode = *receipt.ExitCode
		}
		return failure
	}
	return errors.New("environment initialization operation unconfirmed")
}

// capabilityPreparer is the authenticated Runtime operation; no provider command
// or expanded Skill files cross this boundary.
type capabilityPreparer interface {
	PrepareCapabilities(context.Context, string, proto.CapabilitiesPreparePayload, []byte) (proto.CapabilitiesResultPayload, error)
}

func runRuntimeCapabilities(ctx context.Context, peer capabilityPreparer, owner store.RuntimeAllocation, operation runtimeSetupOperation) error {
	if peer == nil || operation.Capabilities == nil {
		return sandbox.ErrInvalid
	}
	request := *operation.Capabilities
	request.EnvironmentID, request.SessionID = owner.EnvironmentID, owner.SessionID
	result, err := peer.PrepareCapabilities(ctx, uuid.NewString(), request, operation.Archive)
	if err == nil && result.Outcome == "completed" {
		return nil
	}
	if err == nil && (result.Outcome == "rejected" || result.Outcome == "failed") {
		return &runtimeStepFailure{}
	}
	// Keep transport details and Runtime-native text out of public errors.
	return errors.New("environment capability preparation unconfirmed")
}
