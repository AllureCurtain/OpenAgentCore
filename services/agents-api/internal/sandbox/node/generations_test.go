package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/microsandbox"
	"github.com/google/uuid"
)

func generationFixture(count int) *GenerationManager {
	m := &GenerationManager{values: map[uint64]*localGeneration{}, wake: make(chan struct{}, 1)}
	for i := 1; i <= count; i++ {
		g := uint64(i)
		m.values[g] = &localGeneration{value: GenerationProvider{Generation: g, SpecificationDigest: strings.Repeat("a", 64), Provider: &fakeProvider{}}, state: "ready"}
	}
	pin := uint64(1)
	m.target = sandbox.NodeDeployment{Generation: uint64(count), SpecificationDigest: strings.Repeat("a", 64), ServingGeneration: &pin}
	return m
}

func TestSparseGenerationControlHasNoLifetimeLimit(t *testing.T) {
	for _, count := range []int{9, 17, 257} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := generationFixture(count)
			statuses, retained := map[uint64]bool{}, map[uint64]bool{}
			cursor := uint64(0)
			for i := 0; i < count; i++ {
				batch := m.Statuses()
				if len(batch) > 8 || len(batch) < 2 || batch[0].Generation != uint64(count) || batch[1].Generation != 1 {
					t.Fatal("unbounded batch or missing priorities", batch)
				}
				for _, item := range batch {
					statuses[item.Generation] = true
				}
				refs := m.Retained(cursor)
				if len(refs) > 8 {
					t.Fatal("unbounded retention batch")
				}
				for _, ref := range refs {
					retained[ref.Generation] = true
				}
				cursor = refs[len(refs)-1].Generation
			}
			if len(statuses) != count || len(retained) != count || len(m.values) != count {
				t.Fatal("sparse rotation lost generations", len(statuses), len(retained), len(m.values))
			}
		})
	}
}

func TestRetentionReplyMustMatchWholePendingExchange(t *testing.T) {
	for _, mutation := range []string{"id", "sequence", "connection", "epoch", "partial", "reordered", "digest"} {
		t.Run(mutation, func(t *testing.T) {
			m := generationFixture(3)
			a := &agent{config: AgentConfig{Generations: m}}
			c := &agentConnection{id: uuid.NewString(), epoch: 7}
			refs := m.Retained(0)
			c.pending = &generationControl{ID: uuid.NewString(), Sequence: 1, ConnectionID: c.id, OwnerEpoch: c.epoch, References: refs}
			reply := &generationControl{ID: c.pending.ID, Sequence: 1, ConnectionID: c.id, OwnerEpoch: c.epoch}
			for _, ref := range refs {
				reply.Retentions = append(reply.Retentions, sandbox.GenerationRetention{GenerationReference: ref, Keep: false})
			}
			switch mutation {
			case "id":
				reply.ID = uuid.NewString()
			case "sequence":
				reply.Sequence++
			case "connection":
				reply.ConnectionID = uuid.NewString()
			case "epoch":
				reply.OwnerEpoch++
			case "partial":
				reply.Retentions = reply.Retentions[:1]
			case "reordered":
				reply.Retentions[0], reply.Retentions[1] = reply.Retentions[1], reply.Retentions[0]
			case "digest":
				reply.Retentions[0].SpecificationDigest = strings.Repeat("b", 64)
			}
			work := make(chan []sandbox.GenerationRetention, 1)
			f := frame{Control: reply, Deployment: &m.target}
			if err := a.acceptRetention(c, f, work); err == nil {
				t.Fatal("invalid grant accepted")
			}
			if len(work) != 0 || len(m.values) != 3 || c.pending == nil {
				t.Fatal("invalid exchange authorized partial collection")
			}
		})
	}
	m := generationFixture(3)
	a := &agent{config: AgentConfig{Generations: m}}
	c := &agentConnection{id: uuid.NewString(), epoch: 7}
	refs := m.Retained(0)
	c.pending = &generationControl{ID: uuid.NewString(), Sequence: 1, ConnectionID: c.id, OwnerEpoch: c.epoch, References: refs}
	reply := &generationControl{ID: c.pending.ID, Sequence: 1, ConnectionID: c.id, OwnerEpoch: c.epoch}
	for _, ref := range refs {
		reply.Retentions = append(reply.Retentions, sandbox.GenerationRetention{GenerationReference: ref})
	}
	work := make(chan []sandbox.GenerationRetention, 1)
	f := frame{Control: reply, Deployment: &m.target}
	if err := a.acceptRetention(c, f, work); err != nil {
		t.Fatal(err)
	}
	if err := a.acceptRetention(c, f, work); err == nil {
		t.Fatal("replayed grant accepted")
	}
	if len(work) != 1 {
		t.Fatal("grant queued more than once")
	}
}

type helperOwnedProvider struct {
	*fakeProvider
	caller *microsandbox.ProcessCaller
}

func (p *helperOwnedProvider) Quiescent() bool { return p.caller.Quiescent() }

func TestGrantedDropWaitsForReferencesAndActualHelperExit(t *testing.T) {
	root := t.TempDir()
	helper := filepath.Join(root, "helper")
	started := filepath.Join(root, "started")
	release := filepath.Join(root, "release")
	artifact := filepath.Join(root, "runtime")
	// A local test helper uses files only as deterministic process barriers. Its
	// response waiter returns on cancellation while the actual process stays alive.
	script := "#!/bin/sh\ncat >/dev/null\nprintf started > '" + started + "'\nwhile [ ! -f '" + release + "' ]; do sleep 0.01; done\nprintf '{}\\n'\n"
	if err := os.WriteFile(helper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, []byte("retained bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	caller := &microsandbox.ProcessCaller{}
	defer func() { _ = os.WriteFile(release, nil, 0600); wait(t, caller.Quiescent) }()
	m := generationFixture(3)
	m.target.ServingGeneration = nil
	m.values[1].value.Provider = &helperOwnedProvider{fakeProvider: &fakeProvider{}, caller: caller}
	m.options.Remove = func(context.Context, GenerationProvider) error { return os.Remove(artifact) }
	_, _, unref, err := m.Acquire(1)
	if err != nil {
		t.Fatal(err)
	}
	grant := sandbox.GenerationRetention{GenerationReference: sandbox.GenerationReference{Generation: 1, SpecificationDigest: strings.Repeat("a", 64)}}
	if err := m.Drop(t.Context(), grant); !errors.Is(err, ErrUnavailable) {
		t.Fatal("queued reference did not retain bytes", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := caller.Call(ctx, microsandbox.Request{Config: microsandbox.Config{HelperPath: helper}})
		done <- err
	}()
	wait(t, func() bool { _, err := os.Stat(started); return err == nil })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	unref()
	if caller.Quiescent() {
		t.Fatal("caller cancellation pretended helper exited")
	}
	if err := m.Drop(t.Context(), grant); !errors.Is(err, ErrUnavailable) {
		t.Fatal("live helper did not retain generation", err)
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Fatal("granted drop removed live helper bytes", err)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !caller.Quiescent() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !caller.Quiescent() {
		t.Fatal("actual helper Wait did not settle")
	}
	if err := m.Drop(t.Context(), grant); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("settled unreferenced generation was not collected", err)
	}
}
