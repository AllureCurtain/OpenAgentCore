package main

import (
	"context"
	"errors"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/execution"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/runtimeobs"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/e2b"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
)

type generationStore interface {
	GetSandboxAllocationSetup(context.Context, sandbox.Reference) (store.SandboxSetup, error)
	GetSandboxSetup(context.Context) (store.SandboxSetup, error)
	SandboxGenerationPage(context.Context, int64) ([]store.SandboxSetup, error)
	SandboxCredentialAllocationPage(context.Context, string) ([]store.RuntimeAllocation, error)
}

// Every facade, including already running lifecycles, resolves the allocation's
// immutable specification and current credential. There is no mutable provider
// map to unload and no current-generation fallback for a missing historical row.
type e2bGenerationRouter struct {
	setup *managedSetup
	store generationStore
}

func (p *e2bGenerationRouter) route(ctx context.Context, r sandbox.Reference) (sandbox.SandboxProvider, func(), error) {
	release, err := p.setup.e2bCalls.Enter(ctx)
	if err != nil {
		return nil, nil, err
	}
	setup, err := p.store.GetSandboxAllocationSetup(ctx, r)
	if err != nil {
		release()
		return nil, nil, err
	}
	provider, err := p.setup.provider(setup)
	if err != nil {
		release()
		return nil, nil, err
	}
	return provider, release, nil
}
func (p *e2bGenerationRouter) Create(ctx context.Context, b sandbox.Bootstrap) (sandbox.Info, error) {
	v, done, err := p.route(ctx, b.Reference)
	if err != nil {
		return sandbox.Info{}, err
	}
	defer done()
	return v.Create(ctx, b)
}
func (p *e2bGenerationRouter) GetInfo(ctx context.Context, r sandbox.Reference) (sandbox.Info, error) {
	v, done, err := p.route(ctx, r)
	if err != nil {
		return sandbox.Info{}, err
	}
	defer done()
	return v.GetInfo(ctx, r)
}
func (p *e2bGenerationRouter) Renew(ctx context.Context, r sandbox.Reference) (sandbox.Info, error) {
	v, done, err := p.route(ctx, r)
	if err != nil {
		return sandbox.Info{}, err
	}
	defer done()
	return v.Renew(ctx, r)
}
func (p *e2bGenerationRouter) Kill(ctx context.Context, r sandbox.Reference) error {
	v, done, err := p.route(ctx, r)
	if err != nil {
		return err
	}
	defer done()
	return v.Kill(ctx, r)
}
func (p *e2bGenerationRouter) RunCommand(ctx context.Context, r sandbox.Reference, c sandbox.Command) (sandbox.CommandResult, error) {
	v, done, err := p.route(ctx, r)
	if err != nil {
		return sandbox.CommandResult{}, err
	}
	defer done()
	return v.RunCommand(ctx, r, c)
}
func (p *e2bGenerationRouter) Observe(ctx context.Context, t runtimeobs.Target) (runtimeobs.Sample, error) {
	v, done, err := p.route(ctx, sandbox.Reference{TenantID: t.TenantID, EnvironmentID: t.EnvironmentID, AllocationID: t.Instance.AllocationID})
	if err != nil {
		return runtimeobs.Sample{}, err
	}
	defer done()
	return v.(*e2b.Provider).Observe(ctx, t)
}

func (s *managedSetup) routeGenerations(candidate execution.PreparedRuntimeDeployment, setup store.SandboxSetup) (execution.PreparedRuntimeDeployment, error) {
	if setup.Provider != "e2b" {
		return candidate, nil
	}
	db, ok := s.store.(generationStore)
	if !ok {
		return execution.PreparedRuntimeDeployment{}, errors.New("sandbox generation store is unavailable")
	}
	candidate.Config.Provider = &e2bGenerationRouter{setup: s, store: db}
	candidate.FenceCredential = func(ctx context.Context) (func(), error) {
		release, err := s.e2bCalls.Fence(ctx)
		if err != nil {
			return nil, e2b.ErrRequestUnconfirmed
		}
		return release, nil
	}
	candidate.VerifyCredential = func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// Preserve the committed credential until its ownership anchor is verified.
		// Public template readability cannot establish which team owns a deployment.
		verify := func(value store.SandboxSetup, refs []sandbox.Reference) error {
			value.InstallationID = setup.InstallationID
			provider, err := s.provider(value)
			if err != nil {
				return err
			}
			return provider.(*e2b.Provider).VerifyCredential(ctx, refs)
		}
		current, err := db.GetSandboxSetup(ctx)
		if err != nil {
			return err
		}
		if current.Provider != "e2b" || current.E2B == nil {
			return &store.SandboxResetRequiredError{CurrentProvider: current.Provider, RequestedProvider: "e2b"}
		}
		if err := verify(current, nil); err != nil {
			if errors.Is(err, e2b.ErrCredentialInvalid) || errors.Is(err, e2b.ErrTeamMismatch) {
				// A revoked legacy key or a public template outside its team cannot
				// anchor ownership. This says nothing about the candidate key's validity.
				return &store.SandboxResetRequiredError{CurrentProvider: "e2b", RequestedProvider: "e2b"}
			}
			return err
		}
		withCandidateKey := func(value store.SandboxSetup, refs []sandbox.Reference) error {
			value.E2B = &store.SandboxE2BConfiguration{APIKey: setup.E2B.APIKey, Template: value.E2B.Template, APIURL: value.E2B.APIURL, Domain: value.E2B.Domain}
			return verify(value, refs)
		}
		if err := withCandidateKey(current, nil); err != nil {
			return err
		}
		if err := verify(setup, nil); err != nil {
			return err
		}
		generations := map[uint64]store.SandboxSetup{current.Generation: current}
		for after := int64(-1); ; {
			page, err := db.SandboxGenerationPage(ctx, after)
			if err != nil {
				return err
			}
			for _, g := range page {
				generations[g.Generation] = g
				if err := withCandidateKey(g, nil); err != nil {
					return err
				}
				after = int64(g.Generation)
			}
			if len(page) < 32 {
				break
			}
		}
		for after := ""; ; {
			page, err := db.SandboxCredentialAllocationPage(ctx, after)
			if err != nil {
				return err
			}
			refsByGeneration := make(map[uint64][]sandbox.Reference)
			for _, a := range page {
				refsByGeneration[a.DeploymentGeneration] = append(refsByGeneration[a.DeploymentGeneration], sandbox.Reference{TenantID: a.TenantID, EnvironmentID: a.EnvironmentID, AllocationID: a.ID})
				after = a.ID
			}
			for generation, refs := range refsByGeneration {
				owner, ok := generations[generation]
				if !ok {
					return e2b.ErrRequestUnconfirmed
				}
				if err := withCandidateKey(owner, refs); err != nil {
					return err
				}
			}
			if len(page) < 32 {
				break
			}
		}
		return nil
	}
	return candidate, nil
}

// A batch can contain allocations from different endpoint generations. Let the
// observation worker route each allocation through its immutable generation.
func (p *e2bGenerationRouter) ObserveBatch(_ context.Context, _ []runtimeobs.Target) ([]runtimeobs.BatchResult, bool) {
	return nil, false
}
