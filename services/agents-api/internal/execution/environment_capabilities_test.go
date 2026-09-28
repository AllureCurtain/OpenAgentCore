package execution

import (
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"slices"
	"testing"
)

func TestSelfHostedCapabilitySourcesAreFrozenAndStrict(t *testing.T) {
	session := store.Session{ID: "session", TenantID: "tenant"}
	environment := store.Environment{ID: "environment", SessionID: session.ID, TenantID: session.TenantID,
		Configuration: []byte(`{"type":"self_hosted","workspace_directory":"/home/user/project","capability_directories":["/home/user/skills","/opt/plugins"]}`)}
	var request proto.PromptRequestPayload
	if err := (&Dispatcher{}).configurePreparedEnvironment(session, environment, store.ExecutionDevice{EnvironmentID: environment.ID}, &request); err != nil {
		t.Fatal(err)
	}
	local := request.LocalEnvironment
	if local.WorkspaceDirectory != "/home/user/project" || !local.Capabilities || local.ToolEnvironment || len(local.Skills) != 0 ||
		local.CapabilitySources == nil || !slices.Equal(local.CapabilitySources.Directories, []string{"/home/user/skills", "/opt/plugins"}) {
		t.Fatal("frozen source selections lost", local)
	}
	for _, configuration := range []string{
		`{"type":"self_hosted","workspace_directory":"/a/../b"}`,
		`{"type":"self_hosted","workspace_directory":"relative"}`,
		`{"type":"self_hosted","workspace_directory":"/work","capability_directories":["/a","/a"]}`,
		`{"type":"self_hosted","workspace_directory":"/work","skills":[]}`,
		`{"type":"self_hosted","workspace_directory":"/work","initialization":true}`,
	} {
		if LocalWorkspaceConfiguration([]byte(configuration)) {
			t.Fatal("invalid self-hosted shape accepted", configuration)
		}
	}
}
