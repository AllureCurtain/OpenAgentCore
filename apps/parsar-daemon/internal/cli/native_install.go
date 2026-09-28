package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/paths"
	"github.com/MiniMax-AI-Dev/parsar/internal/runtimefs"
)

// Native installation is a user-owned configuration, not an OS service or sandbox.
// The same binary can be run by an operator's preferred service manager.
type nativeInstallation struct {
	Version             string `json:"version"`
	Remote              string `json:"remote_url"`
	Environment         string `json:"environment_id"`
	Workspace           string `json:"workspace_directory"`
	Credential          string `json:"credential_file"`
	CapabilityDirectory string `json:"capability_directory"`
	ToolEnvironmentFile string `json:"tool_environment_file,omitempty"`
}

func nativeInstallationPath() (string, error) {
	root, err := paths.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "daemon", "installation.json"), nil
}

func runInstall(rc *runContext, args []string) error {
	flags := newFlagSet("install")
	remote := flags.String("remote", "", "Environment remote_url from Core")
	environment := flags.String("environment-id", "", "Environment ID from Core")
	workspace := flags.String("workspace", "", "existing absolute workspace directory")
	credential := flags.String("credential-file", "", "absolute executor credential JSON file")
	capabilities := flags.String("capability-directory", "", "installed capability snapshot directory (defaults to Runtime state)")
	toolEnv := flags.String("tool-env-file", "", "optional absolute JSON file with tool environment variables")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("install: unexpected positional arguments")
	}
	if _, err := environmentBase(*remote); err != nil {
		return err
	}
	if !environmentUUID(*environment) {
		return errors.New("install: canonical Environment ID required")
	}
	root, err := paths.Root()
	if err != nil {
		return err
	}
	if *capabilities == "" {
		*capabilities = filepath.Join(root, "capabilities")
	}
	for _, path := range []string{*workspace, *credential, *capabilities} {
		if runtimefs.ValidateLocalPath(path) != nil {
			return errors.New("install: clean absolute paths required")
		}
	}
	info, err := os.Stat(*workspace)
	if err != nil || !info.IsDir() {
		return errors.New("install: existing workspace directory required")
	}
	if _, _, err = executorCredential(*credential, *environment); err != nil {
		return err
	}
	if *toolEnv != "" && runtimefs.ValidateLocalPath(*toolEnv) != nil {
		return errors.New("install: absolute tool environment file required")
	}
	config := nativeInstallation{Version: Version, Remote: *remote, Environment: *environment, Workspace: *workspace, Credential: *credential, CapabilityDirectory: *capabilities, ToolEnvironmentFile: *toolEnv}
	path, err := nativeInstallationPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	held, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer held.Close()
	unlock, err := runtimefs.LockDirectory(held)
	if err != nil {
		return err
	}
	defer unlock()
	if _, err = held.Stat(filepath.Base(path)); err == nil {
		return errors.New("install: installation already exists; in-place upgrades are unsupported, use its current binary or reinstall explicitly")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, _ := json.MarshalIndent(config, "", "  ")
	if err = runtimefs.WritePrivateAtomic(held, filepath.Base(path), raw); err != nil {
		return err
	}
	fmt.Fprintln(rc.stdout, "Installed daemon configuration:", path)
	fmt.Fprintln(rc.stdout, "Run oac-daemon start; use stop, status and logs with the same OAC_RUNTIME_HOME.")
	return nil
}

func runStart(rc *runContext, args []string) error {
	flags := newFlagSet("start")
	foreground := flags.Bool("foreground", false, "stay attached to this terminal")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("start: unexpected positional arguments")
	}
	path, err := nativeInstallationPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	var config nativeInstallation
	if err != nil || decodeEnvironmentJSON(raw, &config) != nil {
		return errors.New("start: installation configuration unavailable; run install first")
	}
	if config.Version != Version {
		return errors.New("start: this installation version is unsupported; reinstall explicitly")
	}
	for key, value := range map[string]string{
		"OAC_RUNTIME_EXECUTION_MODE":       "native",
		"OAC_RUNTIME_WORKSPACE":            config.Workspace,
		"OAC_RUNTIME_CAPABILITY_DIRECTORY": config.CapabilityDirectory,
		"OAC_RUNTIME_TOOL_ENV_FILE":        config.ToolEnvironmentFile,
	} {
		if err = os.Setenv(key, value); err != nil {
			return err
		}
	}
	return runEnvironmentConnect(rc, paths.DefaultProfile, !*foreground, config.Remote, config.Environment, config.Credential, false)
}
