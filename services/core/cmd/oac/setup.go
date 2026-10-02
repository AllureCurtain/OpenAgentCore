package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// setupSandbox saves the default microsandbox deployment after the first start.
// The host command runs it inside the Core network, where the Core key is readable.
func setupSandbox(ctx context.Context, root string, runner Runner) error {
	data := dataDir(root)
	return runner.Run(ctx, "run", "--rm", "--no-deps",
		"-e", "OAC_INNER_SETUP=1",
		"-v", filepath.Join(data, "secrets", "web")+":/run/web:ro",
		"-v", filepath.Join(data, "node-payload")+":/node-payload:ro",
		"--entrypoint", "/usr/local/bin/oac", "core", "setup-sandbox")
}

func setupSandboxInner(ctx context.Context) error {
	key, err := os.ReadFile("/run/web/core.key")
	if err != nil {
		return errors.New("cannot read the Core key")
	}
	activeRaw, err := os.ReadFile("/node-payload/active.json")
	if err != nil {
		return errors.New("node payload is missing")
	}
	var active struct {
		SourceCommit string `json:"source_commit"`
	}
	if json.Unmarshal(activeRaw, &active) != nil || active.SourceCommit == "" {
		return errors.New("node payload identity is invalid")
	}
	base := filepath.Join("/node-payload/releases", active.SourceCommit)
	manifestRaw, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		SourceCommit string `json:"source_commit"`
		Images       struct {
			Runtime string `json:"runtime"`
		} `json:"images"`
		ImageManifestDigests struct {
			Runtime string `json:"runtime"`
		} `json:"image_manifest_digests"`
		RuntimeRef   string `json:"runtime_ref"`
		Microsandbox struct {
			RuntimeSHA  string `json:"runtime_sha256"`
			FirmwareSHA string `json:"firmware_sha256"`
		} `json:"microsandbox"`
	}
	if json.Unmarshal(manifestRaw, &manifest) != nil {
		return errors.New("node manifest is invalid")
	}
	sizesRaw, err := os.ReadFile(filepath.Join(base, "standard-sizes.json"))
	if err != nil {
		return err
	}
	var sizes struct {
		Microsandbox map[string]int `json:"microsandbox"`
	}
	if json.Unmarshal(sizesRaw, &sizes) != nil || sizes.Microsandbox["cpus"] < 1 {
		return errors.New("standard sandbox size is invalid")
	}
	token := string(bytes.TrimSpace(key))
	current, err := coreJSON(ctx, http.MethodGet, token, nil)
	if err != nil {
		return err
	}
	if current["provider"] == "microsandbox" {
		return nil
	}
	if provider, _ := current["provider"].(string); provider != "" {
		return fmt.Errorf("Core already uses %s sandboxes", provider)
	}
	generation, ok := current["generation"].(float64)
	if !ok || generation < 0 {
		return errors.New("Core returned an invalid deployment generation")
	}
	body := map[string]any{
		"provider":            "microsandbox",
		"expected_generation": int(generation),
		"resources":           sizes.Microsandbox,
		"runtime": map[string]string{
			"source_commit":         manifest.SourceCommit,
			"image_id":              manifest.Images.Runtime,
			"image_manifest_digest": manifest.ImageManifestDigests.Runtime,
			"microsandbox_ref":      manifest.RuntimeRef,
			"runtime_sha256":        manifest.Microsandbox.RuntimeSHA,
			"firmware_sha256":       manifest.Microsandbox.FirmwareSHA,
		},
	}
	_, err = coreJSON(ctx, http.MethodPost, token, body)
	return err
}

func coreJSON(ctx context.Context, method, key string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://core:8091/core/v1/sandbox/deployment", reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.New("Core did not confirm the sandbox setup")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("Core refused the sandbox setup")
	}
	var decoded map[string]any
	if json.Unmarshal(raw, &decoded) != nil {
		return nil, errors.New("Core did not confirm the sandbox setup")
	}
	return decoded, nil
}
