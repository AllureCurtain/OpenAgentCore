package localworkspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/google/uuid"
)

func writeSourceSkill(t *testing.T, directory, text string) {
	t.Helper()
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte("---\nname: local-proof\ndescription: Local preparation proof\n---\n"+text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPreparationFreezesLocalContentsAcrossReconnect(t *testing.T) {
	b, req := testBinding(t)
	source := t.TempDir()
	writeSourceSkill(t, source, "first")
	req.LocalEnvironment.Capabilities = true
	req.LocalEnvironment.CapabilitySources = &agentcapabilities.Input{Directories: []string{source}}
	configured, err := b.Configure(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(b.capabilityRoot, agentcapabilities.ManifestName)); !os.IsNotExist(err) {
		t.Fatal("binding installed before asynchronous admission")
	}
	first, err := b.Prepare(t.Context(), configured)
	if err != nil || len(first.LocalEnvironment.Skills) != 1 {
		t.Fatalf("first preparation: %v", err)
	}
	writeSourceSkill(t, source, "second")
	reconnect, err := New(b.environment, b.capabilityIdentity().SessionID, b.workspace, b.helper)
	if err != nil {
		t.Fatal(err)
	}
	reconnect.networkAccess = b.networkAccess
	reconnect.capabilityRoot = b.capabilityRoot
	again, err := reconnect.Prepare(t.Context(), configured)
	if err != nil || len(again.LocalEnvironment.Skills) != 1 {
		t.Fatalf("reconnection: %v", err)
	}
	frozen, err := os.ReadFile(filepath.Join(b.capabilityRoot, again.LocalEnvironment.Skills[0].RelativeRoot, "SKILL.md"))
	if err != nil || string(frozen[len(frozen)-5:]) != "first" {
		t.Fatal("reconnection recaptured source", err)
	}
	next, nextReq := testBinding(t)
	nextReq.LocalEnvironment.Capabilities = true
	nextReq.LocalEnvironment.CapabilitySources = req.LocalEnvironment.CapabilitySources
	nextConfigured, err := next.Configure(nextReq)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = next.Prepare(t.Context(), nextConfigured); err != nil {
		t.Fatal(err)
	}
	fresh, err := os.ReadFile(filepath.Join(next.capabilityRoot, "directories/0/SKILL.md"))
	if err != nil || string(fresh[len(fresh)-6:]) != "second" {
		t.Fatal("new Session did not capture new source", err)
	}
	// Reusing a snapshot under another identity or selection cannot start native work.
	reconnect.stateKey = "agents-api-" + uuid.NewString()
	changed := configured
	changed.AgentStateKey = reconnect.stateKey
	if _, err = reconnect.Prepare(t.Context(), changed); err == nil {
		t.Fatal("foreign snapshot accepted")
	}
	changed = configured
	local := *configured.LocalEnvironment
	local.CapabilitySources = &agentcapabilities.Input{}
	changed.LocalEnvironment = &local
	if _, err = b.Prepare(t.Context(), changed); err == nil {
		t.Fatal("changed selection accepted")
	}
}

func TestPreparationRejectsPrivateSourcesAndLeavesFailuresInert(t *testing.T) {
	b, req := testBinding(t)
	private := t.TempDir()
	t.Setenv("OAC_RUNTIME_HOME", private)
	writeSourceSkill(t, private, "private")
	for _, source := range []string{private, filepath.Dir(private), b.capabilityRoot, InitializationDirectory} {
		if root, err := b.resolveCapabilityDirectory(source); err == nil {
			root.Close()
			t.Fatal("protected source accepted")
		}
	}
	source := filepath.Join(b.workspace, "selected")
	writeSourceSkill(t, source, "valid")
	link := filepath.Join(b.workspace, "link")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if root, err := b.resolveCapabilityDirectory(link); err == nil {
		root.Close()
		t.Fatal("source alias accepted")
	}
	root, err := b.resolveCapabilityDirectory("/workspace/selected")
	if err != nil {
		t.Fatal("logical workspace source rejected", err)
	}
	root.Close()
	req.LocalEnvironment.Capabilities = true
	req.LocalEnvironment.CapabilitySources = &agentcapabilities.Input{Directories: []string{source, t.TempDir()}}
	configured, err := b.Configure(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Prepare(t.Context(), configured); err == nil {
		t.Fatal("invalid second source accepted")
	}
	if _, err = os.Stat(filepath.Join(b.capabilityRoot, agentcapabilities.ManifestName)); !os.IsNotExist(err) {
		t.Fatal("partial snapshot ready")
	}
	if _, err = b.Prepare(t.Context(), configured); err == nil {
		t.Fatal("partial snapshot silently replayed")
	}
}

func TestPreparationEmptySelectionAndCancellation(t *testing.T) {
	b, req := testBinding(t)
	configured, err := b.Configure(req)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err = b.Prepare(ctx, configured); err == nil {
		t.Fatal("cancelled preparation started")
	}
	if _, err = b.Prepare(t.Context(), configured); err != nil {
		t.Fatal(err)
	}
	read := proto.PromptRequestPayload{WorkspaceReadOnly: true}
	if _, err = b.Prepare(t.Context(), read); err != nil {
		t.Fatal("Files required capability installation", err)
	}
}
