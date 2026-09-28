package store_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentplugin"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/MiniMax-AI-Dev/parsar/contracts/agents-api/v1"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/credentialcrypto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"github.com/google/uuid"
)

type initializingProvider struct {
	lifecycleProvider
	initializationPeer
}

func (p *initializingProvider) Create(ctx context.Context, b sandbox.Bootstrap) (sandbox.Info, error) {
	info, err := p.lifecycleProvider.Create(ctx, b)
	if err == nil {
		err = p.connect(b)
	}
	return info, err
}
func (p *initializingProvider) RunCommand(ctx context.Context, r sandbox.Reference, c sandbox.Command) (sandbox.CommandResult, error) {
	return p.initializationPeer.RunCommand(ctx, r, c)
}

func TestManagedInitialFilesGateFairnessCompletionAndRestart(t *testing.T) {
	for _, mode := range []string{"complete", "restart", "uncertain", "setup-complete", "setup-restart", "setup-uncertain"} {
		t.Run(mode, func(t *testing.T) {
			setupOnly := strings.HasPrefix(mode, "setup-")
			mode = strings.TrimPrefix(mode, "setup-")
			expectedSteps := 2
			_, pool := store.NewManagedTestStore(t)
			cipher, err := credentialcrypto.New(bytes.Repeat([]byte{9}, 32))
			if err != nil {
				t.Fatal(err)
			}
			s := store.NewWithCredentialCipher(pool, cipher)
			tenant := uuid.NewString()
			input := store.CreateSessionInput{Creator: store.FixtureCreator(), Engine: "codex", IdempotencyKey: uuid.NewString(), Configuration: json.RawMessage(`{"environment":{"type":"openai_hosted"}}`), InitialFiles: []store.InitialFile{{Type: "inline", Path: "/workspace/a", Data: []byte("first")}, {Type: "inline", Path: "/workspace/b", Data: []byte("second")}}}
			if setupOnly {
				input.InitialFiles = nil
				input.Initialization = store.EnvironmentSetup{Env: map[string]string{"VALUE": "private"}, Packages: v1.EnvironmentPackages{NPM: []string{"is-number@7.0.0"}}, Commands: []store.SetupCommand{{Command: "touch first"}, {Command: "test -f first"}}}
				expectedSteps = 4
			}
			session, err := s.CreateSession(t.Context(), tenant, input)
			if err != nil {
				t.Fatal(err)
			}
			env, err := s.GetSessionEnvironment(t.Context(), tenant, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			p := &initializingProvider{lifecycleProvider: lifecycleProvider{resources: map[string]sandbox.Info{}}, initializationPeer: initializationPeer{deferred: true}}
			key := uuid.NewString()
			w, stop := managedWorker(t, s, key, p)
			owner, err := w.ProvisionEnvironment(t.Context(), tenant, env.ID, key)
			if err != nil || owner.Initialization != "pending" {
				t.Fatal("initialization ownership", owner, err)
			}
			credential, ok, err := s.GetDeviceCredential(t.Context(), owner.DeviceID)
			if err != nil || !ok || credential.ID != owner.DeviceID {
				t.Fatal("pending initialization blocks daemon authentication")
			}
			targetCredential := p.credential
			lastStepGets := 0
			p.apply = func(_ proto.RuntimePreparePayload, _ []byte) proto.RuntimePrepareResultPayload {
				p.mu.Lock()
				defer p.mu.Unlock()
				if p.gets-lastStepGets < 33 {
					t.Fatal("initialization advanced before a full allocation scan", p.gets-lastStepGets)
				}
				lastStepGets = p.gets
				if _, err := s.GetSessionDevice(t.Context(), tenant, session.ID); !errors.Is(err, store.ErrNotFound) {
					t.Fatal("pending file access", err)
				}
				if _, err := s.GetSessionExecutionBinding(t.Context(), tenant, session.ID); !errors.Is(err, store.ErrNotFound) {
					t.Fatal("premature native preparation", err)
				}
				if mode == "uncertain" {
					return proto.RuntimePrepareResultPayload{Outcome: "unknown", ErrorCode: "runtime_preparation_unconfirmed"}
				}
				return completedInitialization(proto.RuntimePreparePayload{}, nil)
			}
			// A full page of other allocations is serviced between initialization steps.
			for range 32 {
				otherTenant, _, otherEnv := managedSession(t, s)
				if _, err := w.ProvisionEnvironment(t.Context(), otherTenant, otherEnv.ID, key); err != nil {
					t.Fatal(err)
				}
			}
			// A missing socket must leave every kind of initialization unclaimed.
			for range 3 {
				if err := w.ReconcileManagedRuntimes(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			pending, err := s.GetRuntimeAllocation(t.Context(), tenant, env.ID)
			if err != nil || pending.Initialization != "pending" || p.writes.Load() != 0 {
				t.Fatal("missing peer claimed initialization", pending, err)
			}
			p.deferred = false
			if err := p.connect(sandbox.Bootstrap{DeviceID: owner.DeviceID, Credential: targetCredential}); err != nil {
				t.Fatal(err)
			}
			for n := 0; int(p.writes.Load()) == 0 && n < 100; n++ {
				if err := w.ReconcileManagedRuntimes(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			if int(p.writes.Load()) != 1 {
				t.Fatal("initialization did not perform one bounded file step", int(p.writes.Load()))
			}
			afterFirst := p.gets
			if mode == "restart" {
				stop()
				w, _ = managedWorker(t, s, key, p)
			}
			if mode == "complete" {
				for n := 0; int(p.writes.Load()) < expectedSteps && n < 100; n++ {
					if err := w.ReconcileManagedRuntimes(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
				got, err := s.GetRuntimeAllocation(t.Context(), tenant, env.ID)
				if err != nil || got.Initialization != "complete" || int(p.writes.Load()) != expectedSteps || p.gets <= afterFirst {
					t.Fatal("completion or maintenance", got, err, int(p.writes.Load()))
				}
				if _, err := s.GetSessionExecutionBinding(t.Context(), tenant, session.ID); err != nil {
					t.Fatal("ready execution still blocked", err)
				}
				stop()
				w, _ = managedWorker(t, s, key, p)
				for range 4 {
					if err := w.ReconcileManagedRuntimes(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
				if int(p.writes.Load()) != expectedSteps {
					t.Fatal("completed initialization replayed")
				}
			} else {
				reconcileManagedState(t, w, s, tenant, env.ID, "released")
				if int(p.writes.Load()) != 1 || p.kills != 1 {
					t.Fatal("uncertain initialization replayed or released twice", int(p.writes.Load()), p.kills)
				}
				failed, err := s.GetEnvironment(t.Context(), tenant, env.ID)
				if err != nil || failed.Status != "failed" {
					t.Fatal("failed initialization exposed", failed, err)
				}
			}
			if p.commandCalls.Load() != 0 {
				t.Fatal("initialization invoked Provider.RunCommand")
			}
		})
	}
}

func TestManagedRuntimePreparationAllOperationsUsePeer(t *testing.T) {
	s := hostedFailureStore(t)
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for path, body := range map[string]string{"proof/.codex-plugin/plugin.json": `{"name":"plugin","description":"A plugin.","skills":"./skills"}`, "proof/skills/example/SKILL.md": "---\nname: plugin-proof\ndescription: A plugin Skill.\n---\nProof."} {
		file, err := writer.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	tenant := uuid.NewString()
	fileBody := bytes.Repeat([]byte("bounded bytes"), 12000)
	session, environment := hostedFailureSession(t, s, tenant, store.CreateSessionInput{
		InitialFiles:   []store.InitialFile{{Type: "inline", Path: "/workspace/first", Data: fileBody}},
		Initialization: store.EnvironmentSetup{Skills: []store.EnvironmentSkill{hostedFailureSkill(t)}, Plugins: []store.EnvironmentPlugin{{Metadata: agentplugin.Metadata{Type: "inline", Name: "plugin", Description: "A plugin."}, Archive: archive.Bytes()}}, Packages: v1.EnvironmentPackages{System: []string{"jq"}, NPM: []string{"is-number@7.0.0"}, Python: []string{"packaging==24.2"}}, Commands: []store.SetupCommand{{Command: "read installed bundles and create directory"}}, CapabilityDirectories: []string{"/workspace/generated"}},
	})
	provider := &initializingProvider{lifecycleProvider: lifecycleProvider{resources: map[string]sandbox.Info{}}}
	var actions []string
	provider.apply = func(request proto.RuntimePreparePayload, data []byte) proto.RuntimePrepareResultPayload {
		if request.SessionID != session.ID || request.EnvironmentID != environment.ID {
			t.Error("Runtime identity changed")
		}
		action := request.Action
		if request.Initialization != nil {
			action = request.Initialization.Action
		}
		if action == "file" && !bytes.Equal(data, fileBody) {
			t.Error("initial bytes changed")
		}
		actions = append(actions, action)
		return completedInitialization(request, data)
	}
	key := uuid.NewString()
	worker, _ := managedWorker(t, s, key, provider)
	if _, err := worker.ProvisionEnvironment(t.Context(), tenant, environment.ID, key); err != nil {
		t.Fatal(err)
	}
	for range 30 {
		if err := worker.ReconcileManagedRuntimes(t.Context()); err != nil {
			t.Fatal(err)
		}
		allocation, err := s.GetRuntimeAllocation(t.Context(), tenant, environment.ID)
		if err != nil {
			t.Fatal(err)
		}
		if allocation.Initialization == "complete" {
			break
		}
	}
	expected := []string{"file", "configure", "skill", "plugin", "system", "npm", "python", "setup", "finalize"}
	if !reflect.DeepEqual(actions, expected) || provider.commandCalls.Load() != 0 {
		t.Fatal("typed ordering or provider isolation", actions, provider.commandCalls.Load())
	}
	allocation, err := s.GetRuntimeAllocation(t.Context(), tenant, environment.ID)
	if err != nil || allocation.Initialization != "complete" {
		t.Fatal("initialization incomplete", allocation, err)
	}
}
