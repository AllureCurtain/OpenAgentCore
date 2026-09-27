package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	providerconfig "github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/config"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/node"
)

func runGenerations(ctx context.Context, configFile, stateDir string) error {
	root := filepath.Dir(configFile)
	if stateDir != filepath.Join(root, "state", "node") {
		return errors.New("generation state must belong to the installed node root")
	}
	stored, err := node.RefreshIdentity(ctx, stateDir)
	if err != nil {
		return err
	}
	base, err := providerconfig.Load(configFile)
	if err != nil {
		return err
	}
	if base.InstallationID != stored.Identity.InstallationID || base.Provider != stored.Identity.Provider || base.Generation != stored.Identity.DeploymentGeneration || base.Specification.Digest(base.Provider) != stored.Identity.SpecificationDigest {
		return sandbox.ErrOwnership
	}
	paths, err := filepath.Glob(filepath.Join(stateDir, "generations", "*.json"))
	if err != nil {
		return err
	}
	paths = append([]string{configFile}, paths...)
	values := map[uint64]node.GenerationProvider{}
	recovery := []sandbox.GenerationReference{}
	seen := map[uint64]bool{}
	closeValues := func() {
		for _, v := range values {
			if v.Close != nil {
				v.Close()
			}
		}
	}
	for _, path := range paths {
		config, err := providerconfig.Load(path)
		if err != nil {
			closeValues()
			return err
		}
		if config.InstallationID != base.InstallationID || config.Provider != base.Provider {
			closeValues()
			return sandbox.ErrOwnership
		}
		if path != configFile && filepath.Base(path) != strconv.FormatUint(config.Generation, 10)+".json" {
			closeValues()
			return sandbox.ErrOwnership
		}
		if _, err := os.Lstat(filepath.Join(stateDir, "generations", strconv.FormatUint(config.Generation, 10)+".dropped")); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			closeValues()
			return err
		}
		if seen[config.Generation] {
			continue
		}
		seen[config.Generation] = true
		value, err := buildGeneration(config, stateDir)
		if errors.Is(err, os.ErrNotExist) {
			recovery = append(recovery, sandbox.GenerationReference{Generation: config.Generation, SpecificationDigest: config.Specification.Digest(config.Provider)})
			continue
		}
		if err != nil {
			closeValues()
			return err
		}
		values[config.Generation] = value
	}
	helper := filepath.Join(root, "generation-preparer.pyz")
	runHelper := func(ctx context.Context, action string, generation uint64, digest string) error {
		command := exec.CommandContext(ctx, "python3", helper, "--installation-id", base.InstallationID, "--generation-action", action, "--generation", strconv.FormatUint(generation, 10), "--specification-digest", digest)
		command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		// Only the preparation/collection process group is canceled. Native sandbox
		// helpers retain their independent allocation-lock completion semantics.
		command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
		command.Stdout = os.Stderr
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("%w: local generation %s did not complete", sandbox.ErrRuntimeDownloadFailed, action)
		}
		return nil
	}
	initial := make([]node.GenerationProvider, 0, len(values))
	for _, v := range values {
		initial = append(initial, v)
	}
	manager, err := node.NewGenerationManager(ctx, node.GenerationManagerOptions{Initial: initial, Recover: recovery,
		Prepare: func(ctx context.Context, generation uint64, digest string) (node.GenerationProvider, error) {
			if err := runHelper(ctx, "prepare", generation, digest); err != nil {
				return node.GenerationProvider{}, err
			}
			config, err := providerconfig.Load(filepath.Join(stateDir, "generations", strconv.FormatUint(generation, 10)+".json"))
			if err != nil {
				return node.GenerationProvider{}, err
			}
			if config.InstallationID != base.InstallationID || config.Provider != base.Provider || config.Generation != generation || config.Specification.Digest(config.Provider) != digest {
				return node.GenerationProvider{}, sandbox.ErrOwnership
			}
			value, err := buildGeneration(config, stateDir)
			if err != nil {
				return value, err
			}
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			return value, value.Probe(probeCtx)
		},
		Remove: func(ctx context.Context, value node.GenerationProvider) error {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			return runHelper(ctx, "collect", value.Generation, value.SpecificationDigest)
		},
	})
	if err != nil {
		closeValues()
		return err
	}
	defer manager.Close()
	return node.Run(ctx, node.AgentConfig{CoreURL: stored.CoreURL, StateDirectory: stateDir, Identity: stored.Identity, Credential: stored.Credential, Generations: manager})
}

func buildGeneration(config providerconfig.Config, stateDir string) (node.GenerationProvider, error) {
	if config.Microsandbox != nil {
		directory := filepath.Join(stateDir, "generations")
		if err := os.MkdirAll(directory, 0700); err != nil {
			return node.GenerationProvider{}, err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
			return node.GenerationProvider{}, sandbox.ErrOwnership
		}
		config.Microsandbox.HelperLeasePath = filepath.Join(directory, strconv.FormatUint(config.Generation, 10)+".lease")
	}
	built, closeProvider, err := providerconfig.Build(config)
	if err != nil {
		return node.GenerationProvider{}, err
	}
	probe := built.Probe
	if micro := config.Microsandbox; micro != nil {
		probe = func(ctx context.Context) error {
			if err := built.Probe(ctx); err != nil {
				return err
			}
			command := exec.CommandContext(ctx, micro.RuntimePath, "image", "inspect", micro.Image, "--format", "json")
			command.Env = append([]string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + os.Getenv("HOME")}, "MSB_BACKEND=local", "MSB_HOME="+micro.RuntimeHome, "MSB_PATH="+micro.RuntimePath, "MSB_LIBKRUNFW_PATH="+micro.FirmwarePath)
			reader, err := command.StdoutPipe()
			if err != nil {
				return sandbox.ErrRuntimeImageUnavailable
			}
			if err := command.Start(); err != nil {
				return sandbox.ErrRuntimeImageUnavailable
			}
			raw, readErr := io.ReadAll(io.LimitReader(reader, 64*1024+1))
			if len(raw) > 64*1024 {
				_ = command.Process.Kill()
			}
			waitErr := command.Wait()
			var image struct{ Digest, Architecture, OS string }
			if readErr != nil || waitErr != nil || len(raw) > 64*1024 || json.Unmarshal(raw, &image) != nil || image.Digest != strings.SplitN(micro.Image, "@", 2)[1] || image.Architecture != "amd64" || image.OS != "linux" {
				return sandbox.ErrRuntimeImageUnavailable
			}
			return nil
		}
	}
	return node.GenerationProvider{Generation: config.Generation, SpecificationDigest: built.SpecificationDigest, Provider: built.Provider, Probe: probe, Close: closeProvider}, nil
}
