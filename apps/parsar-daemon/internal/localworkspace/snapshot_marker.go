package localworkspace

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/paths"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/google/uuid"
)

// snapshotMarker remembers completion outside the installed tree. It contains
// only the operator root, never capability configuration or a second inventory.
type snapshotMarker struct {
	directory *os.Root
	lock      *os.File
	name      string
	root      string
	completed bool
}

func (b *Binding) openSnapshotMarker() (*snapshotMarker, error) {
	identity := b.capabilityIdentity()
	for _, id := range []string{identity.EnvironmentID, identity.SessionID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != id {
			return nil, agentcapabilities.ErrInvalid
		}
	}
	private, err := paths.Root()
	if err != nil || agentcapabilities.ValidateLocalDirectories([]string{private}) != nil || private == "/" ||
		!canonicalExistingParent(private) {
		return nil, agentcapabilities.ErrInvalid
	}
	canonical, err := filepath.EvalSymlinks(private)
	if err != nil || canonical != private {
		return nil, agentcapabilities.ErrInvalid
	}
	current, err := os.OpenRoot(private)
	if err != nil {
		return nil, agentcapabilities.ErrInvalid
	}
	for _, child := range []string{"daemon", "capability-installations"} {
		if !privateSnapshotDirectory(current) {
			current.Close()
			return nil, agentcapabilities.ErrInvalid
		}
		err = current.Mkdir(child, 0700)
		if err != nil && !errors.Is(err, os.ErrExist) {
			current.Close()
			return nil, agentcapabilities.ErrInvalid
		}
		created := err == nil
		info, err := current.Lstat(child)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			current.Close()
			return nil, agentcapabilities.ErrInvalid
		}
		if created {
			parent, openErr := current.Open(".")
			if openErr != nil {
				current.Close()
				return nil, agentcapabilities.ErrInvalid
			}
			syncErr := parent.Sync()
			parent.Close()
			if syncErr != nil {
				current.Close()
				return nil, agentcapabilities.ErrInvalid
			}
		}
		next, err := current.OpenRoot(child)
		current.Close()
		if err != nil {
			return nil, agentcapabilities.ErrInvalid
		}
		current = next
	}
	if !privateSnapshotDirectory(current) {
		current.Close()
		return nil, agentcapabilities.ErrInvalid
	}
	lock, err := current.Open(".")
	if err != nil {
		current.Close()
		return nil, agentcapabilities.ErrInvalid
	}
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		lock.Close()
		current.Close()
		return nil, agentcapabilities.ErrInvalid
	}
	marker := &snapshotMarker{directory: current, lock: lock, name: identity.EnvironmentID + "-" + identity.SessionID + ".json", root: b.capabilityRoot}
	completed, err := marker.read()
	if err != nil {
		marker.close()
		return nil, agentcapabilities.ErrInvalid
	}
	marker.completed = completed
	return marker, nil
}

func privateSnapshotDirectory(root *os.Root) bool {
	file, err := root.Open(".")
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid())
}

func (m *snapshotMarker) read() (bool, error) {
	file, err := m.directory.OpenFile(m.name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || stat.Uid != uint32(os.Getuid()) || stat.Nlink != 1 || info.Size() > 8192 {
		return false, agentcapabilities.ErrInvalid
	}
	// Exact bytes also reject duplicate members, trailing data and extra fields.
	expected, _ := json.Marshal(struct {
		Root string `json:"capability_root"`
	}{m.root})
	actual, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || string(actual) != string(expected)+"\n" {
		return false, agentcapabilities.ErrInvalid
	}
	return true, nil
}

func (m *snapshotMarker) complete() error {
	if m.completed {
		return nil
	}
	data, err := json.Marshal(struct {
		Root string `json:"capability_root"`
	}{m.root})
	if err != nil {
		return agentcapabilities.ErrInvalid
	}
	file, err := m.directory.OpenFile(m.name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return agentcapabilities.ErrInvalid
	}
	_, writeErr := file.Write(append(data, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || m.lock.Sync() != nil {
		return agentcapabilities.ErrInvalid
	}
	m.completed = true
	return nil
}

func (m *snapshotMarker) close() {
	_ = syscall.Flock(int(m.lock.Fd()), syscall.LOCK_UN)
	m.lock.Close()
	m.directory.Close()
}
