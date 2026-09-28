package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/google/uuid"
)

// PrepareCapabilities transfers one archive or finalization selection once. An
// interrupted or unconfirmed transfer reports unknown; callers must not replay it.
func (s *Session) PrepareCapabilities(ctx context.Context, id string, request proto.CapabilitiesPreparePayload, data []byte) (proto.CapabilitiesResultPayload, error) {
	unknown := proto.CapabilitiesResultPayload{Outcome: "unknown", ErrorCode: "capabilities_unconfirmed"}
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil || parsed.String() != id || len(data) > proto.CapabilitiesMaxBytes ||
		(request.Step != "" && request.Step != "begin") {
		return unknown, errors.New("agentdaemon gateway: invalid capability preparation")
	}
	request.Step = "begin"
	if request.Action == "finalize" {
		if len(data) != 0 {
			return unknown, errors.New("agentdaemon gateway: invalid capability finalization")
		}
	} else {
		digest := sha256.Sum256(data)
		expected := hex.EncodeToString(digest[:])
		if (request.SizeBytes != 0 && request.SizeBytes != len(data)) || (request.SHA256 != "" && request.SHA256 != expected) {
			return unknown, errors.New("agentdaemon gateway: capability archive mismatch")
		}
		request.SizeBytes, request.SHA256 = len(data), expected
	}
	if !proto.ValidCapabilitiesPrepareRequest(request) {
		return unknown, errors.New("agentdaemon gateway: invalid capability preparation")
	}
	s.capabilitiesMu.Lock()
	if s.IsClosed() {
		s.capabilitiesMu.Unlock()
		return unknown, ErrSessionClosed
	}
	if len(s.capabilities) != 0 {
		s.capabilitiesMu.Unlock()
		return unknown, errors.New("agentdaemon gateway: capability preparation capacity")
	}
	replies := make(chan proto.Envelope, 1)
	s.capabilities = map[string]chan proto.Envelope{id: replies}
	s.capabilitiesMu.Unlock()
	defer func() { s.capabilitiesMu.Lock(); delete(s.capabilities, id); s.capabilitiesMu.Unlock() }()
	ctx, cancel := context.WithTimeout(ctx, 195*time.Second)
	defer cancel()
	exchange := func(payload proto.CapabilitiesPreparePayload, outcome string, offset int) (proto.CapabilitiesResultPayload, error) {
		env, err := proto.NewEnvelope(proto.TypeCapabilitiesPrepare, id, payload)
		if err != nil {
			return unknown, errors.New("agentdaemon gateway: invalid capability frame")
		}
		encoded, err := json.Marshal(env)
		if err != nil || len(encoded) > proto.CapabilitiesMaxFrameBytes {
			return unknown, errors.New("agentdaemon gateway: invalid capability frame")
		}
		reply, err := s.exchangeChunkFrame(ctx, env, replies)
		if err != nil {
			return unknown, err
		}
		var result proto.CapabilitiesResultPayload
		if len(reply.Payload) > proto.CapabilitiesMaxFrameBytes || reply.DecodePayload(&result) != nil ||
			!proto.ValidCapabilitiesResult(result, outcome, offset, len(data)) {
			return unknown, errors.New("agentdaemon gateway: invalid capability receipt")
		}
		if result.Outcome == "unknown" {
			return result, errors.New("agentdaemon gateway: capability preparation unconfirmed")
		}
		return result, nil
	}
	result, err := exchange(request, "ready", 0)
	if err != nil || result.Outcome != "ready" {
		return result, err
	}
	for offset := 0; offset < len(data); {
		end := min(offset+proto.CapabilitiesChunkBytes, len(data))
		result, err = exchange(proto.CapabilitiesPreparePayload{Step: "chunk", Offset: offset, Data: data[offset:end]}, "received", end)
		if err != nil || result.Outcome != "received" {
			return result, err
		}
		offset = end
	}
	return exchange(proto.CapabilitiesPreparePayload{Step: "commit"}, "completed", 0)
}

func (s *Session) dispatchCapabilities(env proto.Envelope) {
	s.capabilitiesMu.Lock()
	defer s.capabilitiesMu.Unlock()
	if replies := s.capabilities[env.ID]; replies != nil {
		select {
		case replies <- env:
		default:
		}
	}
}

func (s *Session) closeCapabilities() {
	s.capabilitiesMu.Lock()
	defer s.capabilitiesMu.Unlock()
	for id, replies := range s.capabilities {
		close(replies)
		delete(s.capabilities, id)
	}
}
