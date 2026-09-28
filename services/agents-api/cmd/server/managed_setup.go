package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/obs/log"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/execution"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/runtimeobs"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/e2b"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/node"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
)

// managedSetup publishes one immutable selection to execution, bootstrap and
// observation. The database owns the selection; this cache is never a writer.
type managedSetup struct {
	store interface {
		GetSandboxSetup(context.Context) (store.SandboxSetup, error)
		ResolveRuntimeGeneration(context.Context, sandbox.Reference) (string, uint64, error)
	}
	hub            *node.Hub
	installationID string
	// publicURL is OAC_PUBLIC_URL; every sandbox reaches Core through it.
	publicURL string
	selected  atomic.Pointer[managedSelection]
	e2bCalls  e2b.CallFence
}

// Empty selections retain their generation so a delayed provider load cannot
// republish a backend retired by reset.
type managedSelection struct {
	Generation uint64
	Config     *execution.RuntimeProvider
}

func (s *managedSetup) publishSelection(generation uint64, config *execution.RuntimeProvider) *execution.RuntimeProvider {
	next := &managedSelection{Generation: generation, Config: config}
	for {
		current := s.selected.Load()
		if current != nil && current.Generation >= generation {
			return current.Config
		}
		if s.selected.CompareAndSwap(current, next) {
			return config
		}
	}
}
func (s *managedSetup) publish(config *execution.RuntimeProvider) {
	s.publishSelection(config.Generation, config)
}
func (s *managedSetup) publishUnconfigured(generation uint64) { s.publishSelection(generation, nil) }

func (s *managedSetup) load(ctx context.Context) (*execution.RuntimeProvider, error) {
	setup, err := s.store.GetSandboxSetup(ctx)
	if err != nil {
		return nil, err
	}
	if setup.InstallationID != s.installationID {
		return nil, errors.New("sandbox installation does not match setup")
	}
	if setup.Provider == "" {
		return s.publishSelection(setup.Generation, nil), nil
	}
	if selected := s.selected.Load(); selected != nil && selected.Generation >= setup.Generation {
		return selected.Config, nil
	}
	candidate, err := s.configuration(setup)
	if err == nil {
		candidate, err = s.routeGenerations(candidate, setup)
	}
	if err != nil {
		log.Warn(ctx, "Hosted provider is unavailable; administrator recovery remains available", "provider", setup.Provider, "error", err)
		return nil, fmt.Errorf("%w: %v", execution.ErrExecutionUnavailable, err)
	}
	return s.publishSelection(setup.Generation, candidate.Config), nil
}

func (s *managedSetup) prepare(ctx context.Context, setup store.SandboxSetup) (execution.PreparedRuntimeDeployment, error) {
	// E2B guests reach Core from E2B's cloud, over the internet.
	if setup.Provider == "e2b" && store.LoopbackOrigin(s.publicURL) {
		return execution.PreparedRuntimeDeployment{}, store.ErrSandboxPublicURLUnreachable
	}
	candidate, err := s.configuration(setup)
	if err != nil {
		return execution.PreparedRuntimeDeployment{}, err
	}
	if provider, ok := candidate.Config.Provider.(*e2b.Provider); ok {
		build, err := provider.ValidateDeployment(ctx)
		if err != nil {
			if errors.Is(err, e2b.ErrCredentialInvalid) || errors.Is(err, e2b.ErrTeamMismatch) {
				return execution.PreparedRuntimeDeployment{}, err
			}
			if errors.Is(err, sandbox.ErrInvalid) {
				return execution.PreparedRuntimeDeployment{}, e2b.ErrTemplateInvalid
			}
			return execution.PreparedRuntimeDeployment{}, e2b.ErrRequestUnconfirmed
		}
		if setup.Specification.Resources == (sandbox.Resources{}) {
			// Omitted E2B resources take the validated build's CPU and memory.
			setup.Specification.Resources = sandbox.Resources{CPUs: build.CPUs, MemoryMiB: build.MemoryMiB}
			if err := setup.Specification.Validate("e2b"); err != nil {
				return execution.PreparedRuntimeDeployment{}, &store.SandboxConfigurationError{Message: "E2B template build resources are outside the supported sandbox limits; select another build"}
			}
			if candidate, err = s.configuration(setup); err != nil {
				return execution.PreparedRuntimeDeployment{}, err
			}
		}
		candidate.E2BTemplateBuild = &store.SandboxE2BTemplateBuild{Status: build.Status, CPUs: int32(build.CPUs), MemoryMiB: int32(build.MemoryMiB)}
		if build.RootDiskMiB != nil && *build.RootDiskMiB <= math.MaxInt32 {
			disk := int32(*build.RootDiskMiB)
			candidate.E2BTemplateBuild.RootDiskMiB = &disk
		}
	}
	return s.routeGenerations(candidate, setup)
}

// Loading an already committed selection must retain provider access to its
// owned resources, even when a new-template validation would now fail.
func (s *managedSetup) configuration(setup store.SandboxSetup) (execution.PreparedRuntimeDeployment, error) {
	if setup.InstallationID != s.installationID {
		return execution.PreparedRuntimeDeployment{}, errors.New("sandbox installation does not match setup")
	}
	provider, err := s.provider(setup)
	if err != nil {
		return execution.PreparedRuntimeDeployment{}, fmt.Errorf("%w: %v", execution.ErrExecutionUnavailable, err)
	}
	selected := &execution.RuntimeProvider{InstallationID: setup.InstallationID, ProviderKind: setup.Provider, Generation: setup.Generation, Mode: setup.Mode, AdmissionPaused: setup.AdmissionPaused,
		CoreURL: s.publicURL + "/api/v1", BackendFingerprint: setup.BackendFingerprint, Provider: provider}
	if _, supportsCheckpoint := provider.(sandbox.CheckpointProvider); supportsCheckpoint {
		selected.Suspension = &execution.RuntimeSuspensionPolicy{IdleTimeout: time.Duration(setup.IdleSeconds) * time.Second,
			Retention: time.Duration(setup.RetentionSeconds) * time.Second, MaxActive: 4, MaxRetained: 16}
	}
	return execution.PreparedRuntimeDeployment{Config: selected, Publish: s.publish}, nil
}

func (s *managedSetup) ObservationProviderType() string {
	if selected := s.selected.Load(); selected != nil && selected.Config != nil {
		return selected.Config.ProviderKind
	}
	return ""
}

// ObserveBatch delegates to the selected provider's batch read. It reports
// ok=false for providers without one, so each target uses Observe instead.
func (s *managedSetup) ObserveBatch(ctx context.Context, targets []runtimeobs.Target) ([]runtimeobs.BatchResult, bool) {
	selected, err := s.load(ctx)
	if err != nil || selected == nil {
		return nil, false
	}
	source, ok := selected.Provider.(runtimeobs.BatchSource)
	if !ok {
		return nil, false
	}
	return source.ObserveBatch(ctx, targets)
}

func (s *managedSetup) Observe(ctx context.Context, target runtimeobs.Target) (runtimeobs.Sample, error) {
	selected, err := s.load(ctx)
	if err != nil {
		return runtimeobs.Sample{}, err
	}
	if selected == nil {
		return runtimeobs.Sample{}, runtimeobs.ErrUnavailable
	}
	source, ok := selected.Provider.(runtimeobs.Source)
	if !ok {
		return runtimeobs.Sample{}, runtimeobs.ErrUnavailable
	}
	return source.Observe(ctx, target)
}

func (s *managedSetup) provider(setup store.SandboxSetup) (sandbox.SandboxProvider, error) {
	switch setup.Provider {
	case "docker", "microsandbox":
		if s.hub == nil {
			return nil, errors.New("sandbox node transport is unavailable")
		}
		return s.hub.GenerationProvider(setup.Provider, s.store.ResolveRuntimeGeneration), nil
	case "e2b":
		if setup.E2B == nil {
			return nil, errors.New("E2B deployment configuration is unavailable")
		}
		binary := os.Getenv("OAC_E2B_PROVIDER_BIN")
		if binary == "" {
			binary = "/opt/oac/e2b/oac-e2b-provider"
		}
		// Only a candidate that omitted its resources has none; its validation
		// reads them from the template build before the candidate is rebuilt.
		var resources *sandbox.Resources
		if setup.Specification.Resources != (sandbox.Resources{}) {
			resources = &setup.Specification.Resources
		}
		provider, err := e2b.NewWithCaller(e2b.Config{Binary: binary, StateDir: os.Getenv("OAC_E2B_STATE_DIR"),
			Resources: resources, InstallationID: setup.InstallationID, APIKey: setup.E2B.APIKey, Template: setup.E2B.Template, TimeoutSeconds: 3600}, &e2b.ProcessCaller{Fence: &s.e2bCalls})
		if err != nil {
			return nil, errors.New("E2B provider cannot load; check the installed helper and private state directory")
		}
		return provider, nil
	default:
		return nil, errors.New("sandbox provider is unavailable")
	}
}
