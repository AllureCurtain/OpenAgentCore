package proto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentplugin"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentskill"
	"github.com/google/uuid"
)

func capabilityBegin() CapabilitiesPreparePayload {
	return CapabilitiesPreparePayload{Step: "begin", EnvironmentID: uuid.NewString(), SessionID: uuid.NewString(),
		Action: "skill", Skill: &agentskill.Metadata{Type: "inline", Name: "example", Description: "Example"}, SizeBytes: 10, SHA256: strings.Repeat("a", 64)}
}

func TestCapabilitiesRequestValidation(t *testing.T) {
	valid := capabilityBegin()
	if !ValidCapabilitiesPrepareRequest(valid) {
		t.Fatal("valid skill refused")
	}
	for name, mutate := range map[string]func(*CapabilitiesPreparePayload){
		"noncanonical identity": func(p *CapabilitiesPreparePayload) { p.EnvironmentID = "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA" },
		"zero identity":         func(p *CapabilitiesPreparePayload) { p.SessionID = uuid.Nil.String() },
		"begin data":            func(p *CapabilitiesPreparePayload) { p.Data = []byte("x") },
		"begin offset":          func(p *CapabilitiesPreparePayload) { p.Offset = 1 },
		"mixed metadata": func(p *CapabilitiesPreparePayload) {
			p.Plugin = &agentplugin.Metadata{Type: "inline", Name: "example", Description: "Example"}
		},
		"mixed sources":    func(p *CapabilitiesPreparePayload) { p.Sources = &agentcapabilities.Input{} },
		"missing metadata": func(p *CapabilitiesPreparePayload) { p.Skill = nil },
		"wrong metadata type": func(p *CapabilitiesPreparePayload) {
			p.Skill = &agentskill.Metadata{Type: "reference", Name: "example", Description: "Example"}
		},
		"missing description": func(p *CapabilitiesPreparePayload) { p.Skill = &agentskill.Metadata{Type: "inline", Name: "example"} },
		"oversized header": func(p *CapabilitiesPreparePayload) {
			p.Skill = &agentskill.Metadata{Type: "inline", Name: "example", Description: strings.Repeat("x", CapabilitiesMaxFrameBytes)}
		},
		"skill slot":        func(p *CapabilitiesPreparePayload) { p.Slot = 1 },
		"no digest":         func(p *CapabilitiesPreparePayload) { p.SHA256 = "" },
		"uppercase digest":  func(p *CapabilitiesPreparePayload) { p.SHA256 = strings.ToUpper(p.SHA256) },
		"short digest":      func(p *CapabilitiesPreparePayload) { p.SHA256 = "aa" },
		"no archive":        func(p *CapabilitiesPreparePayload) { p.SizeBytes = 0 },
		"oversized archive": func(p *CapabilitiesPreparePayload) { p.SizeBytes = CapabilitiesMaxBytes + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			p := valid
			mutate(&p)
			if ValidCapabilitiesPrepareRequest(p) {
				t.Fatal("invalid begin accepted")
			}
		})
	}
	plugin := valid
	plugin.Action, plugin.Skill, plugin.Plugin, plugin.Slot = "plugin", nil, &agentplugin.Metadata{Type: "inline", Name: "example", Description: "Example"}, 49
	if !ValidCapabilitiesPrepareRequest(plugin) {
		t.Fatal("valid plugin refused")
	}
	plugin.Slot = 50
	if ValidCapabilitiesPrepareRequest(plugin) {
		t.Fatal("out of range slot accepted")
	}
	for _, p := range []CapabilitiesPreparePayload{
		{Step: "chunk", Offset: 0, Data: []byte("x")},
		{Step: "chunk", Offset: CapabilitiesMaxBytes - CapabilitiesChunkBytes, Data: make([]byte, CapabilitiesChunkBytes)},
		{Step: "commit"},
	} {
		if !ValidCapabilitiesPrepareRequest(p) {
			t.Fatal("valid continuation refused", p.Step)
		}
	}
	for _, p := range []CapabilitiesPreparePayload{
		{Step: "chunk", Data: []byte("x"), Action: "skill"},
		{Step: "chunk", Data: []byte("x"), EnvironmentID: valid.EnvironmentID},
		{Step: "chunk", Data: []byte("x"), Skill: valid.Skill},
		{Step: "chunk", Data: []byte("x"), Offset: -1},
		{Step: "chunk", Data: []byte("x"), Offset: CapabilitiesMaxBytes},
		{Step: "chunk", Data: make([]byte, CapabilitiesChunkBytes+1)},
		{Step: "chunk"},
		{Step: "commit", Offset: 1},
		{Step: "commit", Data: []byte("x")},
		{Step: "commit", Sources: &agentcapabilities.Input{}},
	} {
		if ValidCapabilitiesPrepareRequest(p) {
			t.Fatal("invalid continuation accepted", p.Step)
		}
	}
}

func TestCapabilitiesFinalizeRequiresExplicitBoundedSelection(t *testing.T) {
	p := CapabilitiesPreparePayload{Step: "begin", EnvironmentID: uuid.NewString(), SessionID: uuid.NewString(), Action: "finalize", Sources: &agentcapabilities.Input{}}
	if !ValidCapabilitiesPrepareRequest(p) {
		t.Fatal("explicit empty finalization refused")
	}
	for _, mutate := range []func(*CapabilitiesPreparePayload){
		func(p *CapabilitiesPreparePayload) { p.Sources = nil },
		func(p *CapabilitiesPreparePayload) { p.SHA256 = strings.Repeat("a", 64) },
		func(p *CapabilitiesPreparePayload) { p.SizeBytes = 1 },
		func(p *CapabilitiesPreparePayload) {
			p.Sources = &agentcapabilities.Input{Directories: make([]string, 51)}
		},
		func(p *CapabilitiesPreparePayload) {
			p.Sources = &agentcapabilities.Input{Directories: []string{"/workspace/../secret"}}
		},
		func(p *CapabilitiesPreparePayload) {
			p.Sources = &agentcapabilities.Input{Directories: []string{"/workspace/a", "/workspace/a"}}
		},
	} {
		bad := p
		mutate(&bad)
		if ValidCapabilitiesPrepareRequest(bad) {
			t.Fatal("invalid finalization accepted")
		}
	}
}

func TestCapabilitiesResultCannotMisrepresentUncertainty(t *testing.T) {
	for _, r := range []CapabilitiesResultPayload{
		{Outcome: "rejected", ErrorCode: "capabilities_unconfirmed"},
		{Outcome: "unknown", ErrorCode: "capabilities_rejected"},
		{Outcome: "failed", ErrorCode: "private detail"},
		{Outcome: "rejected", ErrorCode: "invalid_request", Offset: 1},
		{Outcome: "unknown", ErrorCode: "capabilities_unconfirmed", SizeBytes: 10},
		{Outcome: "completed", SizeBytes: 10},
		{Outcome: "ready", Offset: 1},
		{Outcome: "ready", SizeBytes: 10},
		{Outcome: "ready", ErrorCode: "invalid_request"},
	} {
		if ValidCapabilitiesResult(r, "ready", 0, 10) {
			t.Fatal("unsafe receipt accepted", r)
		}
	}
	if ValidCapabilitiesResult(CapabilitiesResultPayload{Outcome: "received", Offset: 1}, "received", 2, 10) {
		t.Fatal("wrong offset accepted")
	}
	if ValidCapabilitiesResult(CapabilitiesResultPayload{Outcome: "completed", SizeBytes: 9}, "completed", 0, 10) {
		t.Fatal("wrong size accepted")
	}
	for _, r := range []CapabilitiesResultPayload{
		{Outcome: "rejected", ErrorCode: "invalid_request"},
		{Outcome: "failed", ErrorCode: "capabilities_failed"},
		{Outcome: "unknown", ErrorCode: "capabilities_unconfirmed"},
		{Outcome: "completed", SizeBytes: 10},
	} {
		if !ValidCapabilitiesResult(r, "completed", 0, 10) {
			t.Fatal("safe receipt refused", r)
		}
	}
}

func TestLocalEnvironmentCarriesSelectionButNeverInstalledRoots(t *testing.T) {
	input := &agentcapabilities.Input{Directories: []string{"/workspace/capabilities"}}
	original := LocalEnvironment{ID: uuid.NewString(), WorkspaceDirectory: "/workspace", CapabilitySources: input, Capabilities: true,
		Skills: []agentcapabilities.InstalledSkill{{RelativeRoot: "private-installed-root"}},
		MCP:    []EnvironmentMCP{{PackageRoot: "private-mcp-root"}}}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-") {
		t.Fatal("installed roots leaked to wire")
	}
	var decoded LocalEnvironment
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != original.ID || decoded.WorkspaceDirectory != "/workspace" || !reflect.DeepEqual(decoded.CapabilitySources, input) || !decoded.Capabilities || len(decoded.Skills) != 0 || len(decoded.MCP) != 0 {
		t.Fatal("selection round trip changed", decoded)
	}
}
