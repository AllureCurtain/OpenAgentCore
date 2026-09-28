package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestNativeInstallationPersistsWithoutStartingOrDeleting(t *testing.T) {
	root := t.TempDir()
	t.Setenv("OAC_RUNTIME_HOME", root)
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(root, "credential.json")
	environment := uuid.NewString()
	body, _ := json.Marshal(map[string]string{"key_id": uuid.NewString(), "executor_token": "test-token", "environment_id": environment})
	if err := os.WriteFile(key, body, 0600); err != nil {
		t.Fatal(err)
	}
	output := new(bytes.Buffer)
	rc := &runContext{stdout: output, stderr: output}
	args := []string{"--remote", "ws://localhost:12345/api/v1/agent-daemon/ws", "--environment-id", environment, "--workspace", workspace, "--credential-file", key}
	if err := runInstall(rc, args); err != nil {
		t.Fatal(err)
	}
	path, _ := nativeInstallationPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var installed nativeInstallation
	if json.Unmarshal(raw, &installed) != nil || installed.Environment != environment || installed.Workspace != workspace {
		t.Fatal("installation did not persist its configuration")
	}
	if err := runInstall(rc, args); err == nil {
		t.Fatal("silently replaced existing installation")
	}
	again, _ := os.ReadFile(path)
	if !bytes.Equal(raw, again) {
		t.Fatal("reinstall changed existing data")
	}
	if _, err := os.Stat(key); err != nil {
		t.Fatal("credential file removed")
	}
	installed.Version = "unsupported-old-version"
	invalid, _ := json.Marshal(installed)
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runStart(rc, []string{"--foreground"}); err == nil {
		t.Fatal("old installation accepted")
	}
}
