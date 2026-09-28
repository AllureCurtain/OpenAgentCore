package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/google/uuid"
)

const capabilitiesPrepareTimeout = 120 * time.Second

// Router.mu protects one connection-local transfer. Partial installation data
// belongs to the bound Environment and is never removed by transfer cleanup.
type capabilitiesUpload struct {
	envelope  proto.Envelope
	request   proto.CapabilitiesPreparePayload
	data      []byte
	ready     chan struct{}
	cancel    context.CancelFunc
	finished  bool
	apply     bool
	uncertain bool
}

func (r *Router) handleCapabilitiesPrepare(ctx context.Context, env proto.Envelope) error {
	id, err := uuid.Parse(env.ID)
	if err != nil || id == uuid.Nil || id.String() != env.ID {
		return errors.New("dispatch: invalid capability preparation identity")
	}
	var request proto.CapabilitiesPreparePayload
	if len(env.Payload) > proto.CapabilitiesMaxFrameBytes || env.DecodePayload(&request) != nil || !proto.ValidCapabilitiesPrepareRequest(request) {
		r.mu.Lock()
		pending := r.capabilitiesPrepare != nil && r.capabilitiesPrepare.envelope.ID == env.ID
		if pending && !r.capabilitiesPrepare.finished {
			r.finishCapabilitiesUploadLocked(r.capabilitiesPrepare, false)
		}
		r.mu.Unlock()
		if pending {
			// A late malformed frame cannot report rejection of an earlier commit.
			return errors.New("dispatch: malformed pending capability frame")
		}
		return r.sendCapabilitiesResult(ctx, env.ID, rejectedCapabilities("invalid_request"))
	}
	r.mu.Lock()
	if r.closed || r.suspension != nil {
		r.mu.Unlock()
		return ErrRouterClosed
	}
	if request.Step == "begin" {
		if r.capabilitiesPrepare != nil {
			duplicate := r.capabilitiesPrepare.envelope.ID == env.ID
			r.mu.Unlock()
			if duplicate {
				return errors.New("dispatch: capability preparation already admitted")
			}
			return r.sendCapabilitiesResult(ctx, env.ID, rejectedCapabilities("capabilities_capacity"))
		}
		if r.localWorkspace == nil || !r.localWorkspace.Matches(request.EnvironmentID, request.SessionID) || r.capabilityResourcesBusyLocked() {
			r.mu.Unlock()
			return r.sendCapabilitiesResult(ctx, env.ID, rejectedCapabilities("resource_unavailable"))
		}
		owner, cancel := context.WithTimeout(context.WithoutCancel(ctx), capabilitiesPrepareTimeout)
		u := &capabilitiesUpload{
			envelope: env, request: request, data: make([]byte, 0, request.SizeBytes),
			ready: make(chan struct{}), cancel: cancel,
		}
		r.capabilitiesPrepare = u
		r.shutdownWG.Add(1)
		r.mu.Unlock()
		go r.runCapabilitiesUpload(owner, u, r.localWorkspace.ApplyCapabilities)
		if err := r.sendCapabilitiesResult(ctx, env.ID, proto.CapabilitiesResultPayload{Outcome: "ready"}); err != nil {
			cancel()
			return err
		}
		return nil
	}
	u := r.capabilitiesPrepare
	if u == nil || u.envelope.ID != env.ID {
		r.mu.Unlock()
		return r.sendCapabilitiesResult(ctx, env.ID, rejectedCapabilities("resource_unavailable"))
	}
	if u.finished {
		r.mu.Unlock()
		return errors.New("dispatch: capability preparation body already closed")
	}
	if request.Step == "chunk" && request.Offset == len(u.data) && len(request.Data) <= u.request.SizeBytes-len(u.data) {
		u.data = append(u.data, request.Data...)
		offset := len(u.data)
		r.mu.Unlock()
		if err := r.sendCapabilitiesResult(ctx, env.ID, proto.CapabilitiesResultPayload{Outcome: "received", Offset: offset}); err != nil {
			u.cancel()
			return err
		}
		return nil
	}
	apply := false
	if request.Step == "commit" && len(u.data) == u.request.SizeBytes {
		if u.request.Action == "finalize" {
			apply = true
		} else {
			digest := sha256.Sum256(u.data)
			apply = hex.EncodeToString(digest[:]) == u.request.SHA256
		}
	}
	r.finishCapabilitiesUploadLocked(u, apply)
	r.mu.Unlock()
	return nil
}

func (r *Router) capabilityResourcesBusyLocked() bool {
	if r.workspaceWrite != nil || r.workspaceExport != nil || len(r.workspaceReads) != 0 || len(r.sessions) != 0 || len(r.idle) != 0 || len(r.executors) != 0 {
		return true
	}
	for _, p := range r.preparations {
		if p.owns || p.busy {
			return true
		}
	}
	return false
}

func (r *Router) finishCapabilitiesUploadLocked(u *capabilitiesUpload, apply bool) {
	u.apply, u.finished = apply, true
	close(u.ready)
}

// apply must return only after its local mutations stop. Cancellation requests
// shutdown, but cannot release ownership while that call is still running.
func (r *Router) runCapabilitiesUpload(ctx context.Context, u *capabilitiesUpload, apply func(context.Context, proto.CapabilitiesPreparePayload, []byte) error) {
	defer r.shutdownWG.Done()
	defer u.cancel()
	select {
	case <-u.ready:
	case <-r.shutdownCh:
	case <-ctx.Done():
	}
	r.mu.Lock()
	admitted := u.apply && !r.closed && ctx.Err() == nil
	u.finished = true
	data := u.data
	u.data = nil
	r.mu.Unlock()
	result := rejectedCapabilities("invalid_request")
	if admitted {
		result = capabilitiesApplyResult(apply(ctx, u.request, data), u.request.SizeBytes)
	}
	// Release the potentially large body before waiting on transport delivery.
	data = nil
	r.mu.Lock()
	u.uncertain = result.Outcome == "unknown"
	if !u.uncertain && r.capabilitiesPrepare == u {
		r.capabilitiesPrepare = nil
	}
	r.mu.Unlock()
	// The result has a separate send budget, independent of an installation timeout.
	_ = r.sendCapabilitiesResult(context.WithoutCancel(ctx), u.envelope.ID, result)
}

func rejectedCapabilities(code string) proto.CapabilitiesResultPayload {
	return proto.CapabilitiesResultPayload{Outcome: "rejected", ErrorCode: code}
}

func capabilitiesApplyResult(err error, size int) proto.CapabilitiesResultPayload {
	if err == nil {
		return proto.CapabilitiesResultPayload{Outcome: "completed", SizeBytes: size}
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && errors.Is(err, agentcapabilities.ErrInvalid) {
		return proto.CapabilitiesResultPayload{Outcome: "failed", ErrorCode: "capabilities_failed"}
	}
	return proto.CapabilitiesResultPayload{Outcome: "unknown", ErrorCode: "capabilities_unconfirmed"}
}

func (r *Router) sendCapabilitiesResult(ctx context.Context, id string, result proto.CapabilitiesResultPayload) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	env, err := proto.NewEnvelope(proto.TypeCapabilitiesResult, id, result)
	if err != nil {
		return err
	}
	return r.sender.Send(ctx, env)
}
