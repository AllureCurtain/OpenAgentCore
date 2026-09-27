package microsandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
)

// ProcessCaller never kills a mutating helper on a Core response timeout.
// The helper retains its allocation flock until the SDK mutation settles,
// including after Core exits. Output is still drained and bounded by this waiter.
type ProcessCaller struct {
	active atomic.Int64
	// LeasePath belongs to the permanent node state namespace, not a release.
	// It is never unlinked, including after the generation is collected.
	LeasePath string
}

func (p *ProcessCaller) Quiescent() bool { return p.active.Load() == 0 }

func (p *ProcessCaller) Call(ctx context.Context, q Request) (Response, error) {
	data, e := json.Marshal(q)
	if e != nil || len(data) > MaxRequestBytes {
		return Response{}, errors.New("invalid helper request")
	}
	lease, err := p.acquireLease()
	if err != nil {
		return Response{}, errors.New("generation helper lease unavailable")
	}
	releaseLease := func() {
		if lease != nil {
			_ = lease.Close()
		}
	}
	cmd := exec.Command(q.Config.HelperPath)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = HelperEnvironment(os.Environ(), q.Config)
	if lease != nil {
		cmd.ExtraFiles = []*os.File{lease}
		cmd.Env = append(cmd.Env, "OAC_NODE_GENERATION_LEASE_FD=3")
	}
	stdout := &limitBuffer{limit: MaxResponseBytes}
	stderr := &limitBuffer{limit: MaxOutputBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	p.active.Add(1)
	if e := cmd.Start(); e != nil {
		p.active.Add(-1)
		releaseLease()
		return Response{}, errors.New("helper unavailable")
	}
	done := make(chan error, 1)
	go func() { err := cmd.Wait(); releaseLease(); p.active.Add(-1); done <- err }()
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case err := <-done:
		if err != nil || stdout.exceeded || stderr.exceeded {
			return Response{}, errors.New("helper result unconfirmed")
		}
	}
	var out Response
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&out); e != nil {
		return Response{}, errors.New("invalid helper response")
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return Response{}, errors.New("trailing helper response")
	}
	return out, nil
}

// Strip native selector and credential-bearing inherited configuration. Operator
// proxy settings are not passed into guest environments by this adapter.
func HelperEnvironment(env []string, c Config) []string {
	out := []string{}
	for _, v := range env {
		key, _, _ := strings.Cut(v, "=")
		if key == "HOME" || key == "PATH" || key == "TMPDIR" || key == "LANG" || key == "SSL_CERT_FILE" || key == "SSL_CERT_DIR" {
			out = append(out, v)
		}
	}
	return append(out, "MSB_HOME="+c.RuntimeHome, "MSB_PATH="+c.RuntimePath, "MSB_LIBKRUNFW_PATH="+c.FirmwarePath, "MSB_BACKEND=local", "RUST_LOG=off", "NO_COLOR=1")
}

type limitBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitBuffer) Write(v []byte) (int, error) {
	n := len(v)
	left := b.limit - b.Len()
	if n > left {
		b.exceeded = true
		v = v[:left]
	}
	_, _ = b.Buffer.Write(v)
	return n, nil // Drain excess output without unbounded retention or pipe deadlock.
}

// A shared open-file-description flock survives node exit through the inherited
// helper descriptor. Close only our descriptor; LOCK_UN would also unlock the
// helper's copy. Collection holds the exclusive lock before touching any bytes.
func (p *ProcessCaller) acquireLease() (*os.File, error) {
	if p.LeasePath == "" {
		return nil, nil
	}
	fd, err := syscall.Open(p.LeasePath, syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), p.LeasePath)
	fail := func(err error) (*os.File, error) { _ = file.Close(); return nil, err }
	info, err := file.Stat()
	if err != nil {
		return fail(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		return fail(errors.New("invalid generation lease"))
	}
	if err := syscall.Flock(fd, syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		return fail(err)
	}
	named, err := os.Lstat(p.LeasePath)
	if err != nil || !os.SameFile(info, named) {
		return fail(errors.New("generation lease replaced"))
	}
	if _, err := os.Lstat(strings.TrimSuffix(p.LeasePath, ".lease") + ".dropped"); !os.IsNotExist(err) {
		return fail(errors.New("generation collected or unavailable"))
	}
	return file, nil
}
