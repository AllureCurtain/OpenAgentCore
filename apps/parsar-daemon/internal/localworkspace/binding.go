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
	"github.com/google/uuid"
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
	for _, id := range []string{environment, session} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != id {
			return nil, errors.New("local workspace requires canonical resource identities")
		}
	}
	for _, name := range []string{workspace, helper} {
		if !filepath.IsAbs(name) || filepath.Clean(name) != name || name == "/" || strings.ContainsAny(name, "\x00\r\n\\") {
			return nil, errors.New("local workspace requires clean absolute deployment paths")
		}
	}
	root, err := os.Lstat(workspace)
	if err != nil || !root.IsDir() || root.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("local workspace root must be an existing directory")
	}
	program, err := os.Stat(helper)
	if err != nil || !program.Mode().IsRegular() || program.Mode().Perm()&0111 == 0 || strings.HasPrefix(helper, workspace+string(filepath.Separator)) {
		return nil, errors.New("local workspace helper must be executable outside the workspace")
	}
	return &Binding{environment: environment, stateKey: "agents-api-" + session, workspace: workspace, helper: helper, capabilityRoot: CapabilityDirectory}, nil
}

// NewWithCapabilityDirectory freezes an operator-owned installation layout.
// Core and capability transfer messages cannot choose this destination.
func NewWithCapabilityDirectory(environment, session, workspace, helper, directory string) (*Binding, error) {
	b, err := New(environment, session, workspace, helper)
	if err != nil {
		return nil, err
	}
	if agentcapabilities.ValidateLocalDirectories([]string{directory}) != nil || directory == "/" || containsPath(workspace, directory) || containsPath(directory, workspace) || containsPath(PackageDirectory, directory) || containsPath(directory, PackageDirectory) {
		return nil, agentcapabilities.ErrInvalid
	}
	private, err := paths.Root()
	if err != nil || !filepath.IsAbs(private) || containsPath(private, directory) || containsPath(directory, private) {
		return nil, agentcapabilities.ErrInvalid
	}
	for _, name := range []string{workspace, private, directory} {
		if !canonicalExistingParent(name) {
			return nil, agentcapabilities.ErrInvalid
		}
	}
	b.capabilityRoot = directory
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
	if exportHelper != "" {
		// Reuse the startup executable/root checks; this grants no caller authority.
		if _, err := New(values[0], values[1], values[2], exportHelper); err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(exportHelper)
		if err != nil || resolved != exportHelper {
			return nil, errors.New("workspace exporter must be a canonical executable")
		}
		b.exportHelper = exportHelper
	}
	if writeHelper != "" || staging != "" {
		if err := b.bindWriter(writeHelper, staging); err != nil {
			return nil, err
		}
	}
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
		if present != local.Capabilities || local.SystemPackages && !local.ToolEnvironment {
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
