package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/gateway"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/gorilla/websocket"
)

// initializationPeer exercises the real authenticated gateway and chunk receipts.
// The provider fixture bootstraps its socket; all initialization runs on that peer.
type initializationPeer struct {
	t            *testing.T
	endpoint     string
	registry     *gateway.Registry
	apply        func(proto.RuntimePreparePayload, []byte) proto.RuntimePrepareResultPayload
	writes       atomic.Int32
	commandCalls atomic.Int32
	deferred     bool
	bootstrap    sandbox.Bootstrap
}

func (p *initializationPeer) setRuntimeGateway(t *testing.T, endpoint string, registry *gateway.Registry) {
	p.t, p.endpoint, p.registry = t, endpoint, registry
}
func (p *initializationPeer) connect(b sandbox.Bootstrap) error {
	p.bootstrap = b
	if p.deferred {
		return nil
	}
	c, _, err := websocket.DefaultDialer.Dial(p.endpoint+"?device_id="+b.DeviceID+"&version="+proto.Version, http.Header{"Authorization": {"Bearer " + b.Credential}})
	if err != nil {
		return err
	}
	p.t.Cleanup(func() { _ = c.Close() })
	go func() {
		var request proto.RuntimePreparePayload
		var data []byte
		for {
			var env proto.Envelope
			if c.ReadJSON(&env) != nil {
				return
			}
			if env.Type != proto.TypeRuntimePrepare {
				continue
			}
			var frame proto.RuntimePreparePayload
			if env.DecodePayload(&frame) != nil || !proto.ValidRuntimePrepareRequest(frame) {
				p.t.Error("invalid Runtime frame")
				return
			}
			result := proto.RuntimePrepareResultPayload{}
			switch frame.Step {
			case "begin":
				request = frame
				data = nil
				result.Outcome = "ready"
			case "chunk":
				if frame.Offset != len(data) {
					p.t.Error("unordered initialization bytes")
					return
				}
				data = append(data, frame.Data...)
				result.Outcome = "received"
				result.Offset = len(data)
			case "commit":
				if request.Action == "file" || request.Action == "skill" || request.Action == "plugin" {
					sum := sha256.Sum256(data)
					if request.SizeBytes != len(data) || request.SHA256 != hex.EncodeToString(sum[:]) {
						p.t.Error("initialization digest changed")
						return
					}
				}
				result = p.apply(request, data)
				p.writes.Add(1)
				if result.Outcome == "completed" {
					result.SizeBytes = len(data)
				}
			}
			reply, err := proto.NewEnvelope(proto.TypeRuntimePrepareResult, env.ID, result)
			if err != nil || c.WriteJSON(reply) != nil {
				return
			}
		}
	}()
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		if _, err := p.registry.LookupDevice(b.DeviceID); err == nil {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return context.DeadlineExceeded
}
func (p *initializationPeer) RunCommand(context.Context, sandbox.Reference, sandbox.Command) (sandbox.CommandResult, error) {
	p.commandCalls.Add(1)
	return sandbox.CommandResult{}, sandbox.ErrInvalid
}
func completedInitialization(proto.RuntimePreparePayload, []byte) proto.RuntimePrepareResultPayload {
	return proto.RuntimePrepareResultPayload{Outcome: "completed"}
}
