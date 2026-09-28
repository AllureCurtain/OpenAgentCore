package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentskill"
	"github.com/google/uuid"
)

func skillPreparation() proto.CapabilitiesPreparePayload {
	return proto.CapabilitiesPreparePayload{EnvironmentID: uuid.NewString(), SessionID: uuid.NewString(), Action: "skill",
		Skill: &agentskill.Metadata{Type: "inline", Name: "example", Description: "Example"}}
}

type capabilityOutcome struct {
	result proto.CapabilitiesResultPayload
	err    error
}

func beginCapabilities(s *Session, ctx context.Context, id string, request proto.CapabilitiesPreparePayload, data []byte) <-chan capabilityOutcome {
	done := make(chan capabilityOutcome, 1)
	go func() { r, err := s.PrepareCapabilities(ctx, id, request, data); done <- capabilityOutcome{r, err} }()
	return done
}
func nextCapabilityFrame(t *testing.T, s *Session) proto.Envelope {
	t.Helper()
	select {
	case env := <-s.sendCh:
		return env
	case <-time.After(3 * time.Second):
		t.Fatal("transfer stopped unexpectedly")
		return proto.Envelope{}
	}
}
func finishCapabilities(t *testing.T, done <-chan capabilityOutcome) capabilityOutcome {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("transfer did not settle")
		return capabilityOutcome{}
	}
}
func replyCapabilities(s *Session, id string, result proto.CapabilitiesResultPayload) {
	reply, _ := proto.NewEnvelope(proto.TypeCapabilitiesResult, id, result)
	s.dispatch(reply)
}
func noCapabilityFrame(t *testing.T, s *Session) {
	t.Helper()
	select {
	case env := <-s.sendCh:
		t.Fatal("unexpected extra frame", env.Type)
	default:
	}
}

func TestCapabilitiesTransfersMoreThanFrameLimitAndCorrelates(t *testing.T) {
	s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
	defer s.Close("test")
	data := bytes.Repeat([]byte{0, 255, 3}, 2<<20)
	id := uuid.NewString()
	request := skillPreparation()
	done := beginCapabilities(s, t.Context(), id, request, data)
	var received []byte
	for {
		env := nextCapabilityFrame(t, s)
		encoded, err := json.Marshal(env)
		var p proto.CapabilitiesPreparePayload
		if err != nil || len(encoded) > proto.CapabilitiesMaxFrameBytes || env.ID != id || env.Type != proto.TypeCapabilitiesPrepare || env.DecodePayload(&p) != nil || !proto.ValidCapabilitiesPrepareRequest(p) {
			t.Fatal("invalid frame")
		}
		result := proto.CapabilitiesResultPayload{}
		switch p.Step {
		case "begin":
			digest := sha256.Sum256(data)
			if p.EnvironmentID != request.EnvironmentID || p.SessionID != request.SessionID || p.Skill == nil || *p.Skill != *request.Skill || p.SizeBytes != len(data) || p.SHA256 != hex.EncodeToString(digest[:]) {
				t.Fatal("identity, metadata or digest changed")
			}
			result.Outcome = "ready"
		case "chunk":
			if p.Offset != len(received) {
				t.Fatal("noncontiguous chunks")
			}
			received = append(received, p.Data...)
			result.Outcome, result.Offset = "received", len(received)
		case "commit":
			if !bytes.Equal(received, data) {
				t.Fatal("archive changed")
			}
			result.Outcome, result.SizeBytes = "completed", len(data)
		}
		replyCapabilities(s, uuid.NewString(), result)
		replyCapabilities(s, id, result)
		if p.Step == "commit" {
			break
		}
	}
	result := finishCapabilities(t, done)
	if result.err != nil || result.result.Outcome != "completed" || result.result.SizeBytes != len(data) {
		t.Fatal(result)
	}
	noCapabilityFrame(t, s)
}

func TestCapabilitiesFinalizeTransfersNoArchive(t *testing.T) {
	s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
	defer s.Close("test")
	request := proto.CapabilitiesPreparePayload{EnvironmentID: uuid.NewString(), SessionID: uuid.NewString(), Action: "finalize", Sources: &agentcapabilities.Input{}}
	id := uuid.NewString()
	done := beginCapabilities(s, t.Context(), id, request, nil)
	env := nextCapabilityFrame(t, s)
	var p proto.CapabilitiesPreparePayload
	if env.DecodePayload(&p) != nil || p.Step != "begin" || p.Sources == nil || p.SizeBytes != 0 || p.SHA256 != "" {
		t.Fatal("invalid finalization", p)
	}
	replyCapabilities(s, id, proto.CapabilitiesResultPayload{Outcome: "ready"})
	env = nextCapabilityFrame(t, s)
	if env.DecodePayload(&p) != nil || p.Step != "commit" {
		t.Fatal("finalization sent archive")
	}
	replyCapabilities(s, id, proto.CapabilitiesResultPayload{Outcome: "completed"})
	if result := finishCapabilities(t, done); result.err != nil || result.result.Outcome != "completed" {
		t.Fatal(result)
	}
}

func TestCapabilitiesRefusesConflictingArchiveBeforeSending(t *testing.T) {
	s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
	defer s.Close("test")
	for _, mutate := range []func(*proto.CapabilitiesPreparePayload){
		func(p *proto.CapabilitiesPreparePayload) { p.SHA256 = "incorrect" },
		func(p *proto.CapabilitiesPreparePayload) { p.SizeBytes = 999 },
		func(p *proto.CapabilitiesPreparePayload) { p.Sources = &agentcapabilities.Input{} },
	} {
		request := skillPreparation()
		mutate(&request)
		if result, err := s.PrepareCapabilities(t.Context(), uuid.NewString(), request, []byte("data")); err == nil || result.Outcome != "unknown" {
			t.Fatal("conflict admitted", result, err)
		}
		noCapabilityFrame(t, s)
	}
}

func TestCapabilitiesStopsAtTerminalOrMalformedReceipt(t *testing.T) {
	for name, receipt := range map[string]proto.CapabilitiesResultPayload{
		"rejected":            {Outcome: "rejected", ErrorCode: "capabilities_rejected"},
		"failed":              {Outcome: "failed", ErrorCode: "capabilities_failed"},
		"unknown":             {Outcome: "unknown", ErrorCode: "capabilities_unconfirmed"},
		"premature completed": {Outcome: "completed", SizeBytes: 4},
		"wrong offset":        {Outcome: "ready", Offset: 1},
		"unsafe code":         {Outcome: "rejected", ErrorCode: "private detail"},
	} {
		t.Run(name, func(t *testing.T) {
			s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
			defer s.Close("test")
			id := uuid.NewString()
			done := beginCapabilities(s, t.Context(), id, skillPreparation(), []byte("data"))
			nextCapabilityFrame(t, s)
			replyCapabilities(s, id, receipt)
			result := finishCapabilities(t, done)
			if name == "rejected" || name == "failed" {
				if result.err != nil || result.result != receipt {
					t.Fatal(result)
				}
			} else if result.err == nil || result.result.Outcome != "unknown" {
				t.Fatal("uncertainty lost", result)
			}
			noCapabilityFrame(t, s)
		})
	}
}

func TestCapabilitiesRejectsWrongChunkReceipt(t *testing.T) {
	s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
	defer s.Close("test")
	id := uuid.NewString()
	done := beginCapabilities(s, t.Context(), id, skillPreparation(), []byte("data"))
	nextCapabilityFrame(t, s)
	replyCapabilities(s, id, proto.CapabilitiesResultPayload{Outcome: "ready"})
	nextCapabilityFrame(t, s)
	replyCapabilities(s, id, proto.CapabilitiesResultPayload{Outcome: "received", Offset: 3})
	result := finishCapabilities(t, done)
	if result.err == nil || result.result.Outcome != "unknown" {
		t.Fatal(result)
	}
	noCapabilityFrame(t, s)
}

func TestCapabilitiesConnectionOwnershipAndUnknownInterruption(t *testing.T) {
	for _, closeConnection := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "disconnect"}[closeConnection], func(t *testing.T) {
			s := NewSession(newFakeConn(), "device", "tenant", "test", nil, nil)
			defer s.Close("test")
			ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
			defer cancel()
			done := beginCapabilities(s, ctx, uuid.NewString(), skillPreparation(), []byte("data"))
			nextCapabilityFrame(t, s)
			if _, err := s.PrepareCapabilities(t.Context(), uuid.NewString(), skillPreparation(), []byte("second")); err == nil {
				t.Fatal("concurrent transfer admitted")
			}
			if closeConnection {
				s.Close("lost connection")
			}
			result := finishCapabilities(t, done)
			if result.result.Outcome != "unknown" || result.result.ErrorCode != "capabilities_unconfirmed" {
				t.Fatal("interruption claimed rejection", result)
			}
			expected := error(context.DeadlineExceeded)
			if closeConnection {
				expected = ErrSessionClosed
			}
			if !errors.Is(result.err, expected) {
				t.Fatal(result.err)
			}
			noCapabilityFrame(t, s)
			s.capabilitiesMu.Lock()
			remaining := len(s.capabilities)
			s.capabilitiesMu.Unlock()
			if remaining != 0 {
				t.Fatal("transfer ownership retained")
			}
		})
	}
}
