package microsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "microsandbox-test-agent" {
		var q Request
		if json.NewDecoder(os.Stdin).Decode(&q) != nil {
			os.Exit(2)
		}
		_, _ = (&ProcessCaller{LeasePath: q.Config.RuntimeHome}).Call(context.Background(), q)
		os.Exit(0)
	}
	if filepath.Base(os.Args[0]) == "microsandbox-test-helper" {
		var q Request
		if json.NewDecoder(os.Stdin).Decode(&q) != nil {
			os.Exit(2)
		}
		switch q.Operation {
		case "lease-held":
			// Model the new native helper's entrypoint before any SDK subprocess.
			if os.Getenv("OAC_NODE_GENERATION_LEASE_FD") != "3" {
				os.Exit(4)
			}
			syscall.CloseOnExec(3)
			if os.WriteFile(q.Config.RuntimePath, []byte("started"), 0600) != nil {
				os.Exit(3)
			}
			until := time.Now().Add(15 * time.Second)
			for {
				if _, err := os.Stat(q.Config.RuntimePath + ".release"); err == nil {
					break
				}
				if time.Now().After(until) {
					os.Exit(5)
				}
				time.Sleep(time.Millisecond)
			}

		case "delayed":
			time.Sleep(150 * time.Millisecond)
			if os.WriteFile(q.Config.RuntimePath, []byte("settled"), 0600) != nil {
				os.Exit(3)
			}
		case "oversize":
			fmt.Print(strings.Repeat("x", MaxResponseBytes+1))
			os.Exit(0)
		case "trailing":
			fmt.Print("{\"Version\":1}{}")
			os.Exit(0)
		}
		_ = json.NewEncoder(os.Stdout).Encode(Response{Version: ProtocolVersion})
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func processConfig(t *testing.T) Config {
	t.Helper()
	c := testConfig()
	directory := t.TempDir()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	c.HelperPath = filepath.Join(directory, "microsandbox-test-helper")
	if e = os.Symlink(executable, c.HelperPath); e != nil {
		t.Fatal(e)
	}
	c.RuntimePath = filepath.Join(directory, "settled")
	return c
}
func TestResponseTimeoutDoesNotKillLifecycleOwner(t *testing.T) {
	c := processConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, e := (&ProcessCaller{}).Call(ctx, Request{Operation: "delayed", Config: c})
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("want timeout: %v", e)
	}
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if data, e := os.ReadFile(c.RuntimePath); e == nil && string(data) == "settled" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("helper was killed before settlement")
}
func TestInvalidOrUnboundedHelperOutputIsUnconfirmed(t *testing.T) {
	for _, mode := range []string{"oversize", "trailing"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, e := (&ProcessCaller{}).Call(ctx, Request{Operation: mode, Config: processConfig(t)})
			if e == nil {
				t.Fatal("invalid helper output accepted")
			}
		})
	}
}

func TestGenerationLeaseSurvivesNodeProcessExit(t *testing.T) {
	c := processConfig(t)
	c.RuntimeHome = filepath.Join(filepath.Dir(c.RuntimePath), "1.lease")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	agent := filepath.Join(filepath.Dir(c.RuntimePath), "microsandbox-test-agent")
	if err := os.Symlink(executable, agent); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(Request{Operation: "lease-held", Config: c})
	command := exec.Command(agent)
	command.Stdin = strings.NewReader(string(raw))
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	defer func() {
		_ = os.WriteFile(c.RuntimePath+".release", nil, 0600)
		waitProcessCondition(t, func() bool { return exclusiveLease(c.RuntimeHome) == nil })
	}()
	waitProcessCondition(t, func() bool { _, err := os.Stat(c.RuntimePath); return err == nil })
	before, err := os.Stat(c.RuntimeHome)
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = command.Wait()
	// A new node has no in-memory references, but cannot collect the old helper.
	restarted := &ProcessCaller{LeasePath: c.RuntimeHome}
	if !restarted.Quiescent() {
		t.Fatal("fresh process unexpectedly has references")
	}
	if err := exclusiveLease(c.RuntimeHome); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("live orphan helper lost lease: %v", err)
	}
	if err := os.WriteFile(c.RuntimePath+".release", nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitProcessCondition(t, func() bool { return exclusiveLease(c.RuntimeHome) == nil })
	after, err := os.Stat(c.RuntimeHome)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("lease inode changed", err)
	}
}

func exclusiveLease(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func waitProcessCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("process condition did not settle")
}

func TestGenerationLeaseExcludesCollectionAndDroppedGeneration(t *testing.T) {
	c := processConfig(t)
	lease := filepath.Join(filepath.Dir(c.RuntimePath), "1.lease")
	file, err := os.OpenFile(lease, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	caller := &ProcessCaller{LeasePath: lease}
	if _, err := caller.Call(t.Context(), Request{Operation: "delayed", Config: c}); err == nil {
		t.Fatal("helper started during exclusive collection")
	}
	if err := os.WriteFile(strings.TrimSuffix(lease, ".lease")+".dropped", []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if _, err := caller.Call(t.Context(), Request{Operation: "delayed", Config: c}); err == nil {
		t.Fatal("helper started after payload collection")
	}
	if _, err := os.Stat(c.RuntimePath); !os.IsNotExist(err) {
		t.Fatal("refused helper mutated files", err)
	}
	// Never accept an alternate inode through symlinks or multiply linked files.
	if err := os.Remove(lease); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(c.RuntimePath, lease); err != nil {
		t.Fatal(err)
	}
	if _, err := caller.acquireLease(); err == nil {
		t.Fatal("symlink lease accepted")
	}
}
