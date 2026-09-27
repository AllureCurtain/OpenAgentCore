package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"github.com/google/uuid"
)

type routingSetupStore struct {
	setupStore
	old   store.SandboxSetup
	oldID string
}

func (s *routingSetupStore) GetSandboxAllocationSetup(_ context.Context, ref sandbox.Reference) (store.SandboxSetup, error) {
	value := s.value
	if ref.AllocationID == s.oldID {
		value = s.old
		key := *value.E2B
		key.APIKey = s.value.E2B.APIKey
		value.E2B = &key
	}
	return value, nil
}
func TestE2BRouterKeepsOldSpecificationWithCommittedCredential(t *testing.T) {
	state := t.TempDir()
	if err := os.Chmod(state, 0700); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(t.TempDir(), "helper")
	script := `#!/usr/bin/env python3
import json,sys,pathlib
q=json.load(sys.stdin)
with (pathlib.Path(q['Config']['StateDir'])/'requests').open('a') as f: f.write(json.dumps(q)+'\n')
info=dict(q['Reference'],State='running',ProviderID='owned',CreateSettled=True)
if q['Operation']=='kill': info['State']='absent'
print(json.dumps({'Version':1,'Info':info}))
`
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OAC_E2B_PROVIDER_BIN", helper)
	t.Setenv("OAC_E2B_STATE_DIR", state)
	id := uuid.NewString()
	old := store.SandboxSetup{InstallationID: id, Provider: "e2b", Mode: "direct", Generation: 1, Specification: sandbox.DeploymentSpec{Resources: sandbox.Resources{CPUs: 2, MemoryMiB: 2048}}, E2B: &store.SandboxE2BConfiguration{APIKey: "old-key", Template: "old:" + uuid.NewString()}}
	current := old
	current.Generation = 2
	current.Specification.Resources.CPUs = 4
	current.E2B = &store.SandboxE2BConfiguration{APIKey: "new-key", Template: "new:" + uuid.NewString()}
	ref := sandbox.Reference{TenantID: uuid.NewString(), EnvironmentID: uuid.NewString(), AllocationID: uuid.NewString()}
	db := &routingSetupStore{setupStore: setupStore{value: current}, old: old, oldID: ref.AllocationID}
	setup := &managedSetup{store: db, installationID: id}
	// A facade retained by a generation-one lifecycle still reads current credentials.
	router := &e2bGenerationRouter{setup: setup, store: db}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := router.GetInfo(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := router.Renew(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := router.Kill(ctx, ref); err != nil {
		t.Fatal(err)
	}
	next := ref
	next.AllocationID = uuid.NewString()
	if _, err := router.GetInfo(ctx, next); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(state, "requests"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 {
		t.Fatal(len(lines))
	}
	for i, line := range lines {
		var q struct {
			Config struct {
				APIKey, Template string
				Resources        sandbox.Resources
			}
		}
		if err := json.Unmarshal([]byte(line), &q); err != nil {
			t.Fatal(err)
		}
		expected := old
		if i == 3 {
			expected = current
		}
		if q.Config.APIKey != "new-key" || q.Config.Template != expected.E2B.Template || q.Config.Resources != expected.Specification.Resources {
			t.Fatal("generation or credential mismatch", i)
		}
	}
}
