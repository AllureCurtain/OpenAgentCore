package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	providerconfig "github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox/config"
)

func TestGenerationJournalRestartIdentity(t *testing.T) {
	config := providerconfig.Config{InstallationID: "installation", Generation: 9, Provider: "docker"}
	stateDir := t.TempDir()
	directory := filepath.Join(stateDir, "generations")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	journal := map[string]any{"installation_id": config.InstallationID, "generation": config.Generation, "specification_digest": config.Specification.Digest(config.Provider)}
	for _, suffix := range []string{".collecting", ".dropped"} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join(directory, "9"+suffix)
			if suffix == ".collecting" {
				journal["image_removed"] = true
			} else {
				delete(journal, "image_removed")
			}
			write := func(value map[string]any) {
				raw, _ := json.Marshal(value)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(journal)
			state, err := generationLocalState(config, stateDir)
			if err != nil || state != suffix[1:] {
				t.Fatal(state, err)
			}
			for _, field := range []string{"installation_id", "generation", "specification_digest"} {
				previous := journal[field]
				journal[field] = "foreign"
				write(journal)
				if _, err = generationLocalState(config, stateDir); err == nil {
					t.Fatal("accepted mismatched", field)
				}
				journal[field] = previous
			}
			write(journal)
			if err = os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err = generationLocalState(config, stateDir); err == nil {
				t.Fatal("accepted non-private marker")
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(directory, "foreign")
			raw, _ := json.Marshal(journal)
			if err = os.WriteFile(foreign, raw, 0600); err != nil {
				t.Fatal(err)
			}
			for _, link := range []func(string, string) error{os.Symlink, os.Link} {
				if err = link(foreign, path); err != nil {
					t.Fatal(err)
				}
				if _, err = generationLocalState(config, stateDir); err == nil {
					t.Fatal("accepted linked journal")
				}
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err = syscall.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = generationLocalState(config, stateDir); err == nil {
				t.Fatal("accepted FIFO journal")
			}
			if err = os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
