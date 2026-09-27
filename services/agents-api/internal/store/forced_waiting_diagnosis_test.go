package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/device"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/gateway"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/adminaudit"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/execution"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/runtime"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/store"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Controlled protocol fixtures, not native/model acceptance. Allocation setup
// uses the real execution lease; the real dispatcher consumes a function request
// through the real authenticated WebSocket and persists waiting before archive.
func TestArchiveWaitingHeartbeatCancellationDiagnosis(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "receipt_without_heartbeat", true: "heartbeat_before_receipt"}[heartbeat], func(t *testing.T) {
			s, pool := store.NewManagedTestStore(t)
			lease, err := s.AcquireExecutionLease(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := lease.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			writer := lease.Store()
			installation := uuid.NewString()
			if err := writer.ClaimWebSandboxDeployment(t.Context(), installation); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.InitializeSandboxDeployment(t.Context(), installation, store.SandboxDeploymentSetupRequest{DeploymentSpec: store.SandboxDeploymentTestSpec("e2b"), Provider: "e2b", E2B: &store.SandboxE2BConfiguration{APIKey: "fixture", Template: "runtime:" + uuid.NewString()}}); err != nil {
				t.Fatal(err)
			}
			projectID := uuid.NewString()
			auditCtx := adminaudit.WithSource(t.Context(), adminaudit.Source{CredentialID: "fixture-admin", ProjectID: projectID, RequestID: uuid.NewString(), TraceID: uuid.NewString()})
			project, err := s.CreateProject(auditCtx, projectID, "Archive diagnosis")
			if err != nil {
				t.Fatal(err)
			}
			configuration := strings.Replace(functionConfiguration, `"type":"none"`, `"type":"openai_hosted","network":{"access":"disabled"}`, 1)
			session, err := s.CreateSession(t.Context(), project.TenantID, store.WithFixtureModelProvider(store.CreateSessionInput{Creator: store.FixtureCreator(), Engine: "codex", IdempotencyKey: uuid.NewString(), Configuration: json.RawMessage(configuration)}))
			if err != nil {
				t.Fatal(err)
			}
			secret := uuid.NewString()
			owner, err := writer.ReserveRuntimeAllocation(t.Context(), project.TenantID, session.Environment.ID, installation, device.HashCredential(secret))
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range []func(context.Context, store.RuntimeAllocation) (store.RuntimeAllocation, error){writer.ObserveRuntimeRunning, writer.SettleRuntimeCreation} {
				owner, err = step(t.Context(), owner)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.BindSessionDevice(t.Context(), project.TenantID, session.ID, owner.DeviceID); err != nil {
				t.Fatal(err)
			}
			generation := uuid.NewString()
			if err := writer.ReplaceEnvironmentConnection(t.Context(), project.TenantID, session.Environment.ID, generation); err != nil {
				t.Fatal(err)
			}
			if err := writer.ObserveEnvironmentConnection(t.Context(), project.TenantID, session.Environment.ID, generation, 1, true); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewUnstartedServer(nil)
			wsURL := "ws://" + server.Listener.Addr().String() + "/api/v1/agent-daemon/ws"
			handler, registry, err := runtime.NewGateway(s, wsURL)
			if err != nil {
				t.Fatal(err)
			}
			server.Config.Handler = handler
			server.Start()
			t.Cleanup(func() { runtime.CloseConnections(registry); server.Close() })
			u, _ := url.Parse(wsURL)
			u.RawQuery = url.Values{"device_id": {owner.DeviceID}, "version": {proto.Version}}.Encode()
			conn, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": {"Bearer " + secret}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.Close() })
			h := &dispatchHarness{t: t, s: s, tenant: project.TenantID, session: session, conn: conn, registry: registry, d: &execution.Dispatcher{Store: writer, Registry: registry}}
			capabilities := workerEnvironmentCapabilities()
			capabilities.FunctionTools = true
			h.write("", proto.TypeHeartbeat, proto.HeartbeatPayload{SupportedAgentKinds: []proto.SupportedAgentKind{{Kind: "codex", Available: true, Capabilities: capabilities}}})
			var peer *gateway.Session
			for deadline := time.Now().Add(3 * time.Second); ; {
				peer, err = registry.LookupDevice(owner.DeviceID)
				if err == nil {
					info, _, known := peer.AgentKindStatus("codex")
					if known && info.Capabilities.FunctionTools {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatal("initial heartbeat missing")
				}
				time.Sleep(time.Millisecond)
			}
			pending, err := s.ReserveEnvironmentInput(t.Context(), h.tenant, session.ID, "pending", []store.Input{{Kind: "message", Payload: json.RawMessage(`{"text":"first"}`)}, {Kind: "message", Payload: json.RawMessage(`{"text":"second"}`)}})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := runPreparedDispatch(h, ctx, pending)
			frame := h.read(proto.TypeExecutionPrepare)
			handle := acknowledgePreparation(h, frame.ID)
			start := readyPreparedDispatch(t, h, frame.ID, handle)
			h.write(frame.ID, proto.TypePreparationStatus, proto.PreparationStatusPayload{Handle: handle, Revision: 3, State: "started", RunID: start.RunID})
			input := store.InputReceipt{TurnID: start.RunID}
			h.write(input.TurnID, proto.TypeFunctionCall, proto.FunctionCallPayload{CallID: "pending", Name: "lookup_ticket", Arguments: json.RawMessage(`{"ticket":"42"}`)})
			state := functionState(t, h, 1)
			if state.LastTurn.Status != store.TurnWaiting {
				t.Fatal(state.LastTurn)
			}

			archived, err := writer.ArchiveManagedSession(auditCtx, h.tenant, session.ID, 1)
			if err != nil || archived.State != "cleanup_pending" {
				t.Fatal(archived, err)
			}
			current, err := s.GetTurn(t.Context(), h.tenant, session.ID, input.TurnID)
			if err != nil || current.Status != store.TurnWaiting || current.CancelRequestedAt.IsZero() {
				t.Fatal("archive must request rather than invent cancellation", current, err)
			}
			if _, err := gateway.NewAuthenticator(s).AuthenticateBearer(t.Context(), owner.DeviceID, secret); !errors.Is(err, gateway.ErrAuthUnknownDevice) {
				t.Fatal("archive allowed renewed authority", err)
			}
			rejected, response, dialErr := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": {"Bearer " + secret}})
			if rejected != nil {
				rejected.Close()
			}
			if response != nil {
				response.Body.Close()
			}
			if dialErr == nil || response == nil || response.StatusCode != http.StatusUnauthorized {
				t.Fatal("revoked Runtime reconnected")
			}
			// Explicitly observe cancel delivery before inducing transport loss. This
			// proves even a sent cancellation can lose its receipt; no ticker timing guess.
			var request proto.PromptCancelPayload
			if err := h.read(proto.TypePromptCancel).DecodePayload(&request); err != nil {
				t.Fatal(err)
			}
			if request.DeliveryID == "" {
				t.Fatal("missing cancel delivery identity")
			}
			if heartbeat {
				h.write("", proto.TypeHeartbeat, proto.HeartbeatPayload{})
				select {
				case <-peer.Closed():
				case <-time.After(3 * time.Second):
					t.Fatal("revoked heartbeat did not close peer")
				}
				got := awaitPreparedDispatch(t, result)
				if got.err != nil || got.run.Turn.Status != store.TurnFailed {
					t.Fatal(got.run.Turn.Status, got.err)
				}
				done := got.run.Turn
				var outcome execution.Result
				if err := json.Unmarshal(done.Outcome, &outcome); err != nil || outcome.ErrorCode != "event_stream_incomplete" {
					t.Fatal(string(done.Outcome), err)
				}
			} else {
				h.write(input.TurnID, proto.TypeInteractionDecisionAck, proto.InteractionDecisionAckPayload{DeliveryID: request.DeliveryID, Applied: true, Outcome: &proto.DonePayload{Usage: proto.Usage{InputTokens: 17}, Metadata: map[string]any{proto.DoneMetaAgentSessionID: "cancelled-native"}}})
				got := awaitPreparedDispatch(t, result)
				if got.err != nil || got.run.Turn.Status != store.TurnCancelled {
					t.Fatal(got.run.Turn.Status, got.err)
				}
				done := got.run.Turn
				var outcome execution.Result
				if err := json.Unmarshal(done.Outcome, &outcome); err != nil || outcome.Done.Usage.InputTokens != 17 {
					t.Fatal("receipt lost usage", string(done.Outcome), err)
				}
			}
			var receipts int
			if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM turn_events WHERE turn_id=$1 AND kind='cancel_receipt'", input.TurnID).Scan(&receipts); err != nil {
				t.Fatal(err)
			}
			wantReceipts := 1
			if heartbeat {
				wantReceipts = 0
			}
			if receipts != wantReceipts {
				t.Fatal("durable cancellation receipt mismatch", receipts, wantReceipts)
			}
			// No provider was invoked in either case; the failure therefore does not
			// require compute destruction. The original durable cleanup owner survives.
			allocation, err := s.GetRuntimeAllocation(t.Context(), h.tenant, session.Environment.ID)
			if err != nil || allocation.State != "cleanup_pending" {
				t.Fatal(allocation, err)
			}
			var revoked bool
			if err := pool.QueryRow(t.Context(), "SELECT revoked_at IS NOT NULL FROM devices WHERE id=$1", owner.DeviceID).Scan(&revoked); err != nil || !revoked {
				t.Fatal(revoked, err)
			}
		})
	}
}
