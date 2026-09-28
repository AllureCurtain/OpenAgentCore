//go:build windows

package daemonize

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureBackground(cmd *exec.Cmd) (func(), func() error, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, nil, err
	}
	name := `Local\OpenAgentCore-daemon-` + hex.EncodeToString(nonce[:])
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, nil, err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, nil, err
	}
	attr := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	stopName, _ := windows.UTF16PtrFromString(name)
	event, err := windows.CreateEvent(&attr, 1, 0, stopName)
	if err != nil {
		return nil, nil, err
	}
	readyName, _ := windows.UTF16PtrFromString(name + "-ready")
	ready, err := windows.CreateEvent(&attr, 1, 0, readyName)
	if err != nil {
		windows.CloseHandle(event)
		return nil, nil, err
	}
	clean := func() { windows.CloseHandle(event); windows.CloseHandle(ready) }
	filtered := cmd.Env[:0]
	for _, entry := range cmd.Env {
		if !strings.HasPrefix(entry, stopEventEnv+"=") {
			filtered = append(filtered, entry)
		}
	}
	cmd.Env = append(filtered, stopEventEnv+"="+name)
	// A short-lived installer must not own the detached daemon through its Job.
	// If an enclosing Job forbids breakaway, Start fails instead of losing ownership.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB}
	return clean, func() error {
		status, err := windows.WaitForSingleObject(ready, 60000)
		if err != nil {
			return err
		}
		if status != windows.WAIT_OBJECT_0 {
			return errors.New("daemonize: background child did not establish its stop endpoint")
		}
		return nil
	}, nil
}
