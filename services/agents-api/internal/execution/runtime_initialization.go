package execution

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/gateway"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
)

// Progress is process-local: a recovered running installation is never replayed.
type runtimeInitialization struct {
	owner       store.RuntimeAllocation
	next, count int
	files       int
	operations  []runtimeSetupOperation
	deadline    time.Time
}

func (r *runtimeLifecycle) observeInitialization(ctx context.Context, owner store.RuntimeAllocation) error {
	if owner.Initialization == "complete" {
		return nil
	}
	if owner.Initialization == "running" {
		if r.initializing != nil && r.initializing.owner.ID == owner.ID {
			return nil
		}
		_, err := r.store.RequestRuntimeCleanup(ctx, owner)
		return err
	}
	if r.initializing != nil {
		return nil
	}
	environment, err := r.store.GetEnvironment(ctx, owner.TenantID, owner.EnvironmentID)
	if err != nil {
		return err
	}
	var cfg struct {
		Initialization bool                        `json:"initialization"`
		Files          []store.InitialFileMetadata `json:"files"`
	}
	if json.Unmarshal(environment.Configuration, &cfg) != nil || len(cfg.Files) > 50 {
		_, err = r.store.RequestRuntimeCleanup(ctx, owner)
		return err
	}
	setup, err := r.store.ReadEnvironmentSetup(ctx, owner.TenantID, owner.SessionID)
	if err != nil {
		return err
	}
	operations := setupOperations(setup)
	if len(cfg.Files)+len(operations) == 0 || cfg.Initialization != !setup.Empty() {
		_, err = r.store.RequestRuntimeCleanup(ctx, owner)
		return err
	}
	// No initialization is claimed before the authenticated Runtime is available.
	peer, err := r.initializationPeer(ctx, owner)
	if err != nil || peer == nil {
		return err
	}
	claimed, err := r.store.ClaimRuntimeInitialization(ctx, owner)
	if err != nil {
		return err
	}
	r.initializing = &runtimeInitialization{owner: claimed, count: len(cfg.Files) + len(operations), files: len(cfg.Files), operations: operations, deadline: time.Now().Add(30 * time.Minute)}
	return nil
}

// One bounded operation follows a full maintenance scan; other allocations get serviced between operations.
func (r *runtimeLifecycle) advanceInitialization(ctx context.Context) error {
	active := r.initializing
	if active == nil {
		return nil
	}
	owner, err := r.store.GetRuntimeAllocation(ctx, active.owner.TenantID, active.owner.EnvironmentID)
	if err != nil {
		return err
	}
	if owner.State == "cleanup_pending" || owner.State == "released" || owner.SessionDeleted || owner.Expired {
		r.initializing = nil
		return nil
	}
	if owner.Initialization == "complete" {
		r.initializing = nil
		return nil
	}
	if owner.ID != active.owner.ID || owner.Initialization != "running" {
		r.initializing = nil
		return sandbox.ErrOwnership
	}
	operation, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if time.Now().After(active.deadline) {
		r.initializing = nil
		return sandbox.ErrCommandUnconfirmed
	}
	if err := r.store.CheckExecutionOwnership(operation); err != nil {
		r.initializing = nil
		return err
	}
	peer, err := r.initializationPeer(operation, owner)
	if err == nil && peer == nil {
		return nil
	}
	identity := agentcapabilities.Identity{EnvironmentID: owner.EnvironmentID, SessionID: owner.SessionID}
	step := store.ProvisioningFailure{Step: store.ProvisioningInitialFile}
	if err == nil && active.next < active.files {
		var file store.InitialFileMetadata
		var body []byte
		file, body, err = r.store.ReadInitialEnvironmentFile(operation, owner.TenantID, owner.SessionID, active.next)
		if err == nil {
			err = installInitialFile(operation, peer, identity, file, body)
		}
	} else if err == nil {
		setup := active.operations[active.next-active.files]
		step = setup.provisioningFailure(0)
		err = runRuntimeSetup(operation, peer, identity, setup)
	}
	if err != nil {
		// Clearing the in-memory owner makes the next observation request cleanup,
		// even when the failed operation consumed its entire deadline.
		r.initializing = nil
		var failed *runtimeStepFailure
		if errors.As(err, &failed) {
			// A confirmed failed step records its safe reason now. Unknown effects
			// keep the generic reason recorded by that later cleanup.
			step.ExitCode = failed.exitCode
			record, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if _, cleanupErr := r.store.FailRuntimeInitialization(record, owner, step); cleanupErr != nil {
				return errors.Join(err, cleanupErr)
			}
		}
		return err
	}
	active.next++
	if active.next == active.count {
		_, err = r.store.CompleteRuntimeInitialization(operation, owner)
		r.initializing = nil
		return err
	}
	return nil
}

// Missing authority/socket before sending consumes no initialization operation.
// Once PrepareRuntime is called, unknown effects use the existing cleanup.
func (r *runtimeLifecycle) initializationPeer(ctx context.Context, owner store.RuntimeAllocation) (*gateway.Session, error) {
	if r.registry == nil {
		return nil, nil
	}
	peer, err := authorizedRuntimePeer(ctx, r.store, r.registry, owner.DeviceID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, gateway.ErrSessionClosed) || errors.Is(err, gateway.ErrDeviceNotRegistered) {
		return nil, nil
	}
	return peer, err
}
