//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func execRuntimeMCP(invocation mcpInvocation) error {
	command := exec.Command(invocation.command, invocation.args[1:]...)
	command.Dir, command.Env = invocation.cwd, invocation.env
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return errRuntimeMCP
	}
	return nil
}
