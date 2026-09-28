//go:build linux

package localworkspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"
	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent/clirunner"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func (b *Binding) initializeRuntime(ctx context.Context, input proto.RuntimeInitialization) error {
	payload := struct {
		Version int `json:"version"`
		proto.RuntimeInitialization
	}{Version: 1, RuntimeInitialization: input}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > proto.RuntimePrepareMaxFrameBytes {
		return agentcapabilities.ErrInvalid
	}
	result, err := runInitializationProcess(ctx, "/usr/bin/python3", []string{"-I", "-S", MCPInitializer, "initialize", b.workspace, b.capabilityRoot}, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return decodeInitialization(result)
}

func (b *Binding) installInitialFile(ctx context.Context, input proto.RuntimeInitialFile, data []byte) (err error) {
	if b.writer == nil || !strings.HasPrefix(input.Path, "/workspace/") || len(data) > proto.RuntimePrepareMaxBytes {
		return agentcapabilities.ErrInvalid
	}
	w := b.writer
	if !w.mu.TryLock() {
		return agentcapabilities.ErrInvalid
	}
	defer w.mu.Unlock()
	if w.uncertain {
		return ErrInitializationUnconfirmed
	}
	defer func() { w.uncertain = errors.Is(err, ErrInitializationUnconfirmed) }()
	digest := sha256.Sum256(data)
	result, err := runInitializationProcess(ctx, "/usr/bin/python3", []string{"-I", "-S", "-c", initialFileInstaller,
		b.workspace, strings.TrimPrefix(input.Path, "/workspace/"), strconv.Itoa(len(data)), w.staging, w.helper},
		io.MultiReader(bytes.NewReader(data), bytes.NewReader(digest[:])))
	if err != nil {
		return err
	}
	if result.exit != 0 || len(result.stderr) != 0 {
		return ErrInitializationUnconfirmed
	}
	_, err = decodeWrite(result.stdout, len(data))
	if errors.Is(err, agent.ErrWorkspaceWriteRejected) {
		return &InitializationFailure{}
	}
	if err != nil {
		return ErrInitializationUnconfirmed
	}
	return nil
}

// This only creates fd-anchored parents, then delegates replacement to the
// existing atomic writer. Deployment paths come exclusively from Binding.
const initialFileInstaller = `import os, sys
workspace, path, size, staging, helper = sys.argv[1:]
parts = path.split('/')
if any(not p or p in ('.', '..') or any(c in p for c in ('\\', '\x00', '\r', '\n')) for p in parts):
    raise SystemExit(2)
flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
fd = os.open('/', flags)
for component in workspace.lstrip('/').split('/'):
    child = os.open(component, flags, dir_fd=fd)
    os.close(fd)
    fd = child
for component in parts[:-1]:
    try:
        child = os.open(component, flags, dir_fd=fd)
    except FileNotFoundError:
        os.mkdir(component, mode=0o700, dir_fd=fd)
        child = os.open(component, flags, dir_fd=fd)
    os.close(fd)
    fd = child
os.close(fd)
os.execv(helper, [helper, workspace, path, size, staging])
`

type initializationResult struct {
	stdout, stderr []byte
	exit           int
}

// runInitializationProcess uses the existing process-group owner and joins all
// readers/writers before returning. No child output becomes a public error.
func runInitializationProcess(ctx context.Context, binary string, args []string, input io.Reader) (initializationResult, error) {
	result := initializationResult{exit: -1}
	operation, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if operation.Err() != nil {
		return result, ErrInitializationUnconfirmed
	}
	process, err := clirunner.Start(clirunner.StartOptions{Parent: operation, Binary: binary, Args: args,
		Dir: "/", Env: []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}, NeedStdin: true,
		OwnProcessGroup: true, KillTimeout: 250 * time.Millisecond})
	if err != nil {
		return result, ErrInitializationUnconfirmed
	}
	defer process.Cancel()
	inputDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(process.Stdin, input)
		closeErr := process.Stdin.Close()
		if err == nil {
			err = closeErr
		}
		inputDone <- err
	}()
	type output struct {
		data []byte
		err  error
	}
	read := func(stream io.Reader, done chan<- output) {
		raw, err := io.ReadAll(io.LimitReader(stream, 1025))
		if err != nil || len(raw) > 1024 {
			process.Cancel()
		}
		_, _ = io.Copy(io.Discard, stream)
		done <- output{raw, err}
	}
	stdout, stderr := make(chan output, 1), make(chan output, 1)
	go read(process.Stdout, stdout)
	go read(process.Stderr, stderr)
	out, diagnostic := <-stdout, <-stderr
	inputErr := <-inputDone
	_ = process.Wait()
	result.stdout, result.stderr = out.data, diagnostic.data
	result.exit = process.Cmd.ProcessState.ExitCode()
	if operation.Err() != nil || inputErr != nil || out.err != nil || diagnostic.err != nil ||
		len(out.data) > 1024 || len(diagnostic.data) > 1024 || result.exit < 0 {
		return result, ErrInitializationUnconfirmed
	}
	return result, nil
}

func decodeInitialization(result initializationResult) error {
	var receipt struct {
		Version  int    `json:"version"`
		Outcome  string `json:"outcome"`
		ExitCode *int   `json:"exit_code"`
	}
	decoder := json.NewDecoder(bytes.NewReader(result.stdout))
	decoder.DisallowUnknownFields()
	if len(result.stderr) != 0 || decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || receipt.Version != 1 {
		return ErrInitializationUnconfirmed
	}
	if result.exit == 0 && receipt.Outcome == "completed" && receipt.ExitCode == nil {
		return nil
	}
	if result.exit == 1 && receipt.Outcome == "failed" {
		if receipt.ExitCode != nil && (*receipt.ExitCode < 1 || *receipt.ExitCode > 255) {
			return ErrInitializationUnconfirmed
		}
		return &InitializationFailure{ExitCode: receipt.ExitCode}
	}
	return ErrInitializationUnconfirmed
}
