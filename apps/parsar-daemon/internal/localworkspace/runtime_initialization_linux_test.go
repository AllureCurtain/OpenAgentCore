//go:build linux

package localworkspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func TestRuntimeInitializationSafeReceipts(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		exit   int
		failed bool
		code   int
	}{
		{`{"version":1,"outcome":"completed"}`, 0, false, 0},
		{`{"version":1,"outcome":"failed","exit_code":3}`, 1, true, 3},
		{`{"version":1,"outcome":"failed"}`, 1, true, 0},
	} {
		err := decodeInitialization(initializationResult{stdout: []byte(tc.raw), exit: tc.exit})
		var failure *InitializationFailure
		if tc.failed {
			if !errors.As(err, &failure) || tc.code != 0 && (failure.ExitCode == nil || *failure.ExitCode != tc.code) {
				t.Fatal("failed receipt", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{
		`{"version":1,"outcome":"failed","exit_code":0}`,
		`{"version":1,"outcome":"failed","exit_code":256}`,
		`{"version":1,"outcome":"failed","exit_code":3,"stderr":"secret"}`,
		`{"version":1,"outcome":"completed","exit_code":3}`,
		`{"version":1,"outcome":"failed"} {}`,
	} {
		if !errors.Is(decodeInitialization(initializationResult{stdout: []byte(raw), exit: 1}), ErrInitializationUnconfirmed) {
			t.Fatal("unsafe receipt")
		}
	}
}

func TestRuntimeInitializationProcessDrainsAndCancels(t *testing.T) {
	t.Setenv("RUNTIME_TEST_PRIVATE", "do-not-inherit")
	result, err := runInitializationProcess(t.Context(), "/bin/sh", []string{"-c", `test -z "$RUNTIME_TEST_PRIVATE" || exit 4; cat >/dev/null; printf '{"version":1,"outcome":"completed"}'`}, strings.NewReader("{}"))
	if err != nil || decodeInitialization(result) != nil {
		t.Fatal("clean process", err)
	}
	_, err = runInitializationProcess(t.Context(), "/bin/sh", []string{"-c", "head -c 2048 /dev/zero; sleep 5"}, strings.NewReader(""))
	if !errors.Is(err, ErrInitializationUnconfirmed) {
		t.Fatal("unbounded output accepted", err)
	}
	directory := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := runInitializationProcess(ctx, "/bin/sh", []string{"-c", `(trap '' TERM; sleep 1; touch "$1/late") & touch "$1/started"; wait`, "fixture", directory}, strings.NewReader("{}"))
		done <- err
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("child did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, ErrInitializationUnconfirmed) {
		t.Fatal("cancellation confirmed", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(directory, "late")); !os.IsNotExist(err) {
		t.Fatal("child survived return", err)
	}
}

func TestRuntimeInitializationPreservesSetupNetworkPhase(t *testing.T) {
	for _, policy := range []string{"disabled", "restricted"} {
		t.Run(policy, func(t *testing.T) {
			b, _ := testBinding(t)
			b.networkAccess = policy
			t.Setenv("OAC_RUNTIME_NETWORK_ACCESS", policy)
			// Initialization's typed network remains independent of native policy.
			script := "import json, os, sys; request=json.load(sys.stdin); assert request['network']=='enabled'; assert 'OAC_RUNTIME_NETWORK_ACCESS' not in os.environ; print('{\"version\":1,\"outcome\":\"completed\"}')"
			result, err := runInitializationProcess(t.Context(), "/usr/bin/python3", []string{"-I", "-S", "-c", script},
				strings.NewReader(`{"version":1,"action":"setup","network":"enabled","command":"true","cwd":"/workspace"}`))
			if err != nil || decodeInitialization(result) != nil || b.NetworkPolicy().Access != policy {
				t.Fatal("initialization changed native network policy", err)
			}
			marker, err := b.openSnapshotMarker()
			if err != nil {
				t.Fatal(err)
			}
			if err = marker.complete(); err != nil {
				marker.close()
				t.Fatal(err)
			}
			marker.close()
			input := proto.RuntimePreparePayload{Step: "begin", EnvironmentID: b.environment, SessionID: b.capabilityIdentity().SessionID,
				Action: "initialize", Initialization: &proto.RuntimeInitialization{Action: "setup", Network: "enabled", Command: "true", CWD: "/workspace"}}
			if err = b.ApplyRuntimePreparation(t.Context(), input, nil); !errors.Is(err, agentcapabilities.ErrInvalid) {
				t.Fatal("initialization reopened after native admission", err)
			}
		})
	}
}

func TestRuntimeInitialFileUsesBoundAtomicWriterAndAnchoredParents(t *testing.T) {
	b := writableBinding(t, `[ "$#" = 4 ] || exit 7
[ -z "$RUNTIME_TEST_PRIVATE" ] || exit 8
cat >/dev/null
printf '{"version":1,"outcome":"completed","size_bytes":3}'
`)
	t.Setenv("RUNTIME_TEST_PRIVATE", "secret")
	if err := b.installInitialFile(t.Context(), proto.RuntimeInitialFile{Path: "/workspace/nested/child/file"}, []byte("abc")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(b.workspace, "nested/child"))
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("parents", err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(b.workspace, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := b.installInitialFile(t.Context(), proto.RuntimeInitialFile{Path: "/workspace/escape/new/file"}, []byte("abc")); err == nil {
		t.Fatal("alias accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatal("escaped workspace")
	}
}

func TestRuntimePreparationRejectsFilesAfterFinalization(t *testing.T) {
	b, req := testBinding(t)
	configured, err := b.Configure(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Prepare(t.Context(), configured); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(nil)
	input := proto.RuntimePreparePayload{Step: "begin", EnvironmentID: b.environment, SessionID: b.capabilityIdentity().SessionID,
		Action: "file", File: &proto.RuntimeInitialFile{Path: "/workspace/file"}, SHA256: hex.EncodeToString(digest[:])}
	if err = b.ApplyRuntimePreparation(t.Context(), input, nil); !errors.Is(err, agentcapabilities.ErrInvalid) {
		t.Fatal("finalized Runtime accepted file", err)
	}
	input.Action = "initialize"
	input.File = nil
	input.SHA256 = ""
	input.Initialization = &proto.RuntimeInitialization{Action: "configure", Env: map[string]string{}}
	if err = b.ApplyRuntimePreparation(t.Context(), input, nil); !errors.Is(err, agentcapabilities.ErrInvalid) {
		t.Fatal("finalized Runtime accepted initialize", err)
	}
}

func TestRuntimeInitialFileNativeAtomicReplacement(t *testing.T) {
	helper := os.Getenv("OAC_TEST_LOCAL_WRITE_HELPER")
	if helper == "" {
		t.Skip("requires built native installer")
	}
	b := writableBinding(t, "exit 1\n")
	if err := b.bindWriter(helper, b.writer.staging); err != nil {
		t.Fatal(err)
	}
	path := "/workspace/n1/n2/file"
	for _, body := range [][]byte{nil, []byte("first"), bytes.Repeat([]byte("x"), 1<<20), []byte("replacement")} {
		if err := b.installInitialFile(t.Context(), proto.RuntimeInitialFile{Path: path}, body); err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(b.workspace, "n1/n2/file"))
		if err != nil || !bytes.Equal(actual, body) {
			t.Fatal("atomic replacement", err)
		}
	}
}
