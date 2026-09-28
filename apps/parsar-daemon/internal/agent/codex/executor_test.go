package codex

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func executorFixture(t *testing.T, mode string) (*Executor, string) {
	t.Helper()
	req, cfg, root := preparationFixture(t)
	t.Setenv("OAC_TEST_EXECUTOR_MODE", mode)
	e, err := newExecutor(t.Context(), req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return e, root
}
func awaitExecutorTurn(t *testing.T, turn agent.Turn, out <-chan proto.Envelope) agent.TurnSettlement {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	settlement, err := turn.AwaitSettlement(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for frame := range out {
		if frame.Type == proto.TypeDone {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("terminal count %d", count)
	}
	return settlement
}
func TestExecutorNormalTurnsKeepProcessAndThread(t *testing.T) {
	e, root := executorFixture(t, "complete")
	var previous agent.Turn
	for _, id := range []string{"one", "two"} {
		out := make(chan proto.Envelope, 20)
		turn, err := e.StartTurn(t.Context(), id, proto.TextInput("answer"), out)
		if err != nil {
			t.Fatal(err)
		}
		if !awaitExecutorTurn(t, turn, out).Reusable {
			t.Fatal("healthy turn was not reusable")
		}
		if previous != nil {
			if err := previous.Cancel(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		previous = turn
	}
	counts := map[string]int{}
	pids := map[int]bool{}
	for _, f := range preparationFrames(t, root) {
		counts[f.Method]++
		pids[f.PID] = true
	}
	if counts["initialize"] != 1 || counts["thread/start"] != 1 || counts["turn/start"] != 2 || counts["turn/interrupt"] != 0 || len(pids) != 1 {
		t.Fatal(counts, pids)
	}
	if !e.prepared.session.rpc.Alive() {
		t.Fatal("normal completion closed executor")
	}
}
func TestExecutorCancellationSettlesThenReuses(t *testing.T) {
	e, root := executorFixture(t, "complete")
	out := make(chan proto.Envelope, 20)
	first, err := e.StartTurn(t.Context(), "cancel", proto.TextInput("hold"), out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartTurn(t.Context(), "overlap", proto.TextInput("answer"), make(chan proto.Envelope, 20)); err == nil {
		t.Fatal("overlapping turn admitted")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := first.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if !awaitExecutorTurn(t, first, out).Reusable {
		t.Fatal("cancelled healthy executor unavailable")
	}
	secondOut := make(chan proto.Envelope, 20)
	second, err := e.StartTurn(t.Context(), "next", proto.TextInput("hold"), secondOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	// A stale handle cannot interrupt its successor.
	interrupts := 0
	for _, f := range preparationFrames(t, root) {
		if f.Method == "turn/interrupt" {
			interrupts++
		}
	}
	if interrupts != 1 {
		t.Fatalf("stale cancellation sent %d interrupts", interrupts)
	}
	if err := second.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if !awaitExecutorTurn(t, second, secondOut).Reusable {
		t.Fatal("second cancellation did not settle")
	}
}
func TestExecutorStartErrorsRetainExactOwnership(t *testing.T) {
	for _, mode := range []string{"start-error", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			e, _ := executorFixture(t, mode)
			out := make(chan proto.Envelope, 20)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			turn, err := e.StartTurn(ctx, "before", proto.TextInput("answer"), out)
			if turn != nil || !errors.Is(err, context.Canceled) {
				t.Fatal(turn, err)
			}
			select {
			case <-out:
				t.Fatal("pre-admission output was retained or closed")
			default:
			}
			turn, err = e.StartTurn(t.Context(), "after", proto.TextInput("answer"), out)
			if turn == nil || err == nil {
				t.Fatal("uncertain submission lost owner", err)
			}
			if awaitExecutorTurn(t, turn, out).Reusable {
				t.Fatal("failed submission was reusable")
			}
		})
	}
}

func TestExecutorCloseRetainsPlanUntilReaped(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestJSONRPCClientFakeCodexProcess$", "--")
	cmd.Env = append(os.Environ(), "CODEX_RPC_FAKE_PROCESS=1", "GORACE=atexit_sleep_ms=0")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	rpc := NewJSONRPCClient(JSONRPCConfig{})
	rpc.cmd, rpc.stdin, rpc.alive = cmd, stdin, true
	var reap sync.Once
	t.Cleanup(func() { _ = cmd.Process.Kill(); reap.Do(rpc.waitChild) })
	_, cancel := context.WithCancel(t.Context())
	var cleaned atomic.Bool
	e := &Executor{prepared: &Prepared{session: &Session{rpc: rpc, cancelFn: cancel}, plan: SessionPlan{Cleanup: func() { cleaned.Store(true) }}}}
	ctx, stop := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer stop()
	if err = e.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("unreaped close", err)
	}
	if cleaned.Load() {
		t.Fatal("plan released before cleanup settled")
	}
	reap.Do(rpc.waitChild)
	if err = e.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !cleaned.Load() {
		t.Fatal("cleanup retry did not release plan")
	}
}
