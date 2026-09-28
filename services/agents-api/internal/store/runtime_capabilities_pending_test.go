package store_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/credentialcrypto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"github.com/google/uuid"
)

func TestManagedCapabilitiesWaitBeforeInitializationClaim(t *testing.T) {
	_, pool := store.NewManagedTestStore(t)
	cipher, err := credentialcrypto.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	s := store.NewWithCredentialCipher(pool, cipher)
	tenant := uuid.NewString()
	session, err := s.CreateSession(t.Context(), tenant, store.CreateSessionInput{
		Creator: store.FixtureCreator(), Engine: "codex", IdempotencyKey: uuid.NewString(),
		Configuration:  json.RawMessage(`{"environment":{"type":"openai_hosted"}}`),
		Initialization: store.EnvironmentSetup{CapabilityDirectories: []string{"/workspace/generated"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := s.GetSessionEnvironment(t.Context(), tenant, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	provider := &initializingProvider{lifecycleProvider: lifecycleProvider{resources: map[string]sandbox.Info{}}}
	key := uuid.NewString()
	worker, _ := managedWorker(t, s, key, provider)
	owner, err := worker.ProvisionEnvironment(t.Context(), tenant, env.ID, key)
	if err != nil || owner.Initialization != "pending" {
		t.Fatal(owner, err)
	}
	for range 4 {
		if err := worker.ReconcileManagedRuntimes(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	owner, err = s.GetRuntimeAllocation(t.Context(), tenant, env.ID)
	if err != nil || owner.Initialization != "pending" || owner.State != "running" || provider.writes != 0 || provider.kills != 0 {
		t.Fatal("missing socket consumed initialization or requested cleanup", owner, err, provider.writes, provider.kills)
	}
	if _, err := s.GetSessionExecutionBinding(t.Context(), tenant, session.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("ordinary readiness gate bypassed", err)
	}
}

func TestSelfHostedCapabilityConfigurationNeedsNoManagedSetup(t *testing.T) {
	s, pool := store.NewManagedTestStore(t)
	tenant := uuid.NewString()
	configuration := json.RawMessage(`{"environment":{"type":"self_hosted","workspace_directory":"/home/user/project","capability_directories":["/opt/skills"]}}`)
	session, err := s.CreateSession(t.Context(), tenant, store.CreateSessionInput{Creator: store.FixtureCreator(), Engine: "codex", IdempotencyKey: uuid.NewString(), Configuration: configuration})
	if err != nil {
		t.Fatal(err)
	}
	environment, err := s.GetSessionEnvironment(t.Context(), tenant, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	var frozen struct {
		Workspace   string   `json:"workspace_directory"`
		Directories []string `json:"capability_directories"`
	}
	if json.Unmarshal(environment.Configuration, &frozen) != nil || frozen.Workspace != "/home/user/project" || len(frozen.Directories) != 1 || frozen.Directories[0] != "/opt/skills" {
		t.Fatal("frozen paths lost")
	}
	var setupCount int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM environment_setups WHERE session_id=$1", session.ID).Scan(&setupCount); err != nil || setupCount != 0 {
		t.Fatal("self-hosted initialization row created", setupCount, err)
	}
	if _, err := s.GetRuntimeAllocation(t.Context(), tenant, environment.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("self-hosted managed allocation created", err)
	}
}
