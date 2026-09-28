package cli

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/localworkspace"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
)

var errRuntimeMCP = errors.New("environment MCP unavailable")

type mcpInvocation struct {
	command string
	args    []string
	cwd     string
	env     []string
}

// runRuntimeMCP forwards stdio to the selected installed MCP server.
func runRuntimeMCP(_ *runContext, args []string) error {
	if len(args) != 3 || args[0] == "/" || agentcapabilities.ValidateLocalDirectories([]string{args[0]}) != nil {
		return errRuntimeMCP
	}
	root, err := os.OpenRoot(args[0])
	if err != nil {
		return errRuntimeMCP
	}
	manifest, err := agentcapabilities.Load(root)
	root.Close()
	if err != nil {
		return errRuntimeMCP
	}
	values, err := localworkspace.ReadOptionalToolEnvironment()
	if err != nil {
		return errRuntimeMCP
	}
	invocation, err := resolveMCPInvocation(manifest, args[0], args[1], args[2], values)
	if err != nil {
		return errRuntimeMCP
	}
	return execRuntimeMCP(invocation)
}

func resolveMCPInvocation(manifest agentcapabilities.Manifest, installationRoot, pkg, name string, values map[string]string) (mcpInvocation, error) {
	if installationRoot == "/" || agentcapabilities.ValidateLocalDirectories([]string{installationRoot}) != nil {
		return mcpInvocation{}, errRuntimeMCP
	}
	for _, installed := range manifest.MCP {
		server := installed.Server
		if installed.PackageRoot != pkg || server.Name != name {
			continue
		}
		if server.Type != "stdio" {
			return mcpInvocation{}, errRuntimeMCP
		}
		// MCP runs under the same user environment as the daemon. Explicit
		// capability variables override inherited values.
		env := map[string]string{}
		for _, entry := range os.Environ() {
			key, value, ok := strings.Cut(entry, "=")
			if ok {
				env[key] = value
			}
		}

		for _, key := range server.EnvVars {
			value, exists := values[key]
			if !exists || strings.ContainsRune(value, 0) {
				return mcpInvocation{}, errRuntimeMCP
			}
			env[key] = value
		}
		cwd := server.CWD
		if !filepath.IsAbs(cwd) {
			cwd = filepath.Join(installationRoot, installed.PackageRoot, cwd)
		}
		result := mcpInvocation{command: server.Command, args: append([]string{server.Command}, server.Args...), cwd: cwd}
		for key, value := range env {
			result.env = append(result.env, key+"="+value)
		}
		sort.Strings(result.env)
		return result, nil
	}
	return mcpInvocation{}, errRuntimeMCP
}
