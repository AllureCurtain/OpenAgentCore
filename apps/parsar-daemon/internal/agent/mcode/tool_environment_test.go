package mcode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSelfHostedToolEnvironment(t *testing.T) {
	config, req, _ := workspaceFixture(t)
	file := filepath.Join(t.TempDir(), "tool-env.json")
	if err := os.WriteFile(file, []byte(`{"USER_VALUE":"ready"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_RUNTIME_TOOL_ENV_FILE", file)
	opts, err := prepareWorkspaceOptions(t.Context(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(opts.DataDir, "workspace-profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		ToolEnv map[string]string `json:"toolEnv"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.ToolEnv["USER_VALUE"] != "ready" {
		t.Fatal("self-hosted tool configuration was not applied")
	}
	if err := os.WriteFile(file, []byte(`[]`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWorkspaceOptions(t.Context(), config, req); err == nil {
		t.Fatal("invalid explicit tool configuration was ignored")
	}
}
