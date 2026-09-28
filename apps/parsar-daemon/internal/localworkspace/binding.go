package localworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/paths"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentnetwork"
)

// Binding freezes operator-owned identity and paths for one Runtime lifetime.
type Binding struct {
	environment    string
	networkAccess  string
	allowedDomains []string
	stateKey       string
	workspace      string
	helper         string
	exportHelper   string
	writer         *fileWriter
	capabilityMu   sync.Mutex
	capabilityRoot string
}

func New(environment, session, workspace, helper string) (*Binding, error) {
	root, err := paths.Root()
	if err != nil {
		return nil, err
	}
	b, err := newNativeBinding(environment, session, workspace, filepath.Join(root, "capabilities"))
	if err != nil {
		return nil, err
	}
	b.helper = helper
	return b, nil
}

// NewWithCapabilityDirectory freezes paths selected by the Runtime operator.
func NewWithCapabilityDirectory(environment, session, workspace, helper, directory string) (*Binding, error) {
	b, err := newNativeBinding(environment, session, workspace, directory)
	if err != nil {
		return nil, err
	}
	b.helper = helper
	return b, nil
}

func Load() (*Binding, error) {
	values := []string{os.Getenv("OAC_RUNTIME_ENVIRONMENT_ID"), os.Getenv("OAC_RUNTIME_SESSION_ID"), os.Getenv("OAC_RUNTIME_WORKSPACE"), os.Getenv("OAC_RUNTIME_DIRECTORY_HELPER")}
	policy, err := RuntimeNetworkPolicy()
	if err != nil {
		return nil, err
	}
	network := policy.Access
	writeHelper, staging := os.Getenv("OAC_RUNTIME_WRITE_HELPER"), os.Getenv("OAC_RUNTIME_STAGING")
	exportHelper := os.Getenv("OAC_RUNTIME_EXPORT_HELPER")
	capabilityDirectory := os.Getenv("OAC_RUNTIME_CAPABILITY_DIRECTORY")
	if strings.Join(values, "") == "" && writeHelper == "" && staging == "" && network == "" && exportHelper == "" && capabilityDirectory == "" {
		return nil, nil
	}
	if capabilityDirectory == "" {
		capabilityDirectory = CapabilityDirectory
	}
	b, err := NewWithCapabilityDirectory(values[0], values[1], values[2], values[3], capabilityDirectory)
	if err != nil {
		return nil, err
	}
	b.networkAccess = network
	b.allowedDomains = policy.Hosts()
	b.exportHelper = exportHelper
	b.writer.helper, b.writer.staging = writeHelper, staging

	return b, nil
}

// Configure validates the reference before supplying the immutable local cwd.
func (b *Binding) Configure(r proto.PromptRequestPayload) (proto.PromptRequestPayload, error) {
	if b == nil && r.LocalEnvironment == nil {
		return r, nil
	}
	if b == nil || r.LocalEnvironment == nil || r.LocalEnvironment.ID != b.environment || r.AgentStateKey != b.stateKey ||
		r.DisableExecutionEnvironment || r.WorkDir != "" ||
		r.ConversationID != "" || r.WorkspaceAuthoring || !r.StrictResume {
		return r, errors.New("request does not match the dedicated local Environment")
	}
	if !r.WorkspaceReadOnly || r.LocalEnvironment.NetworkAccess != "" || len(r.LocalEnvironment.AllowedDomains) > 0 {
		requested := agentnetwork.Policy{Access: r.LocalEnvironment.NetworkAccess, AllowedDomains: r.LocalEnvironment.AllowedDomains}
		if !b.NetworkPolicy().Equal(requested) {
			return r, errors.New("request does not match the local Runtime network policy")
		}
	}
	if !r.WorkspaceReadOnly {
		local := *r.LocalEnvironment
		if local.WorkspaceDirectory != "/workspace" && local.WorkspaceDirectory != b.workspace {
			return r, errors.New("request does not match the local workspace selection")
		}
		if local.CapabilitySources == nil || agentcapabilities.ValidateInput(*local.CapabilitySources) != nil {
			return r, agentcapabilities.ErrInvalid
		}
		sources := *local.CapabilitySources
		present := len(sources.Skills)+len(sources.Plugins)+len(sources.Directories) > 0
		if present != local.Capabilities {
			return r, agentcapabilities.ErrInvalid
		}
		local.Skills, local.MCP, local.CapabilityRoot = nil, nil, ""
		r.LocalEnvironment = &local
		r.WorkDir = b.workspace
	}
	return r, nil
}

func (b *Binding) Matches(environment, session string) bool {
	return b != nil && b.environment == environment && b.stateKey == "agents-api-"+session
}

// Inspect existing ancestors before creating an operator-owned path. This prevents
// MkdirAll from creating directories through an alias before its final validation.
func canonicalExistingParent(name string) bool {
	for {
		if _, err := os.Lstat(name); err == nil {
			actual, err := filepath.EvalSymlinks(name)
			return err == nil && actual == name
		} else if !errors.Is(err, os.ErrNotExist) {
			return false
		}
		parent := filepath.Dir(name)
		if parent == name {
			return false
		}
		name = parent
	}
}
