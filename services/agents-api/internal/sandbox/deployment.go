package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationError preserves the sandbox error text and identity while identifying
// a fixed configuration field and, for numeric limits, fixed inclusive bounds.
type ValidationError struct {
	Param    string
	Min, Max *uint32
	Message  string
}

func (e *ValidationError) Error() string   { return e.Message }
func (e *ValidationError) Unwrap() error   { return ErrInvalid }
func validationBound(value uint32) *uint32 { return &value }

// Resources describes one managed sandbox, independently of node concurrency.
// Disk bounds are available only where the native provider enforces them.
type Resources struct {
	CPUs               uint32 `json:"cpus"`
	MemoryMiB          uint32 `json:"memory_mib"`
	RootDiskMiB        uint32 `json:"root_disk_mib,omitempty"`
	EnvironmentDiskMiB uint32 `json:"environment_disk_mib,omitempty"`
}

func (r Resources) Validate(provider string) error {
	if r.CPUs == 0 || r.CPUs > 255 || r.MemoryMiB < 512 || r.MemoryMiB > 1048576 {
		field, min, max := "resources.cpus", uint32(1), uint32(255)
		if r.CPUs > 0 && r.CPUs <= 255 {
			field, min, max = "resources.memory_mib", 512, 1048576
		}
		return &ValidationError{Param: field, Min: &min, Max: &max, Message: fmt.Sprintf("%s: cpus must be 1..255 and memory_mib must be 512..1048576", ErrInvalid)}
	}
	switch provider {
	case "microsandbox":
		if r.RootDiskMiB < 1024 || r.EnvironmentDiskMiB < 1024 {
			field := "resources.root_disk_mib"
			if r.RootDiskMiB >= 1024 {
				field = "resources.environment_disk_mib"
			}
			return &ValidationError{Param: field, Min: validationBound(1024), Message: fmt.Sprintf("%s: microsandbox requires root_disk_mib and environment_disk_mib of at least 1024 MiB", ErrInvalid)}
		}
	case "docker", "e2b":
		if r.RootDiskMiB != 0 || r.EnvironmentDiskMiB != 0 {
			field := "resources.root_disk_mib"
			if r.RootDiskMiB == 0 {
				field = "resources.environment_disk_mib"
			}
			return &ValidationError{Param: field, Min: validationBound(0), Max: validationBound(0), Message: fmt.Sprintf("%s: %s does not support independent disk capacity limits", ErrInvalid, provider)}
		}
	default:
		return fmt.Errorf("%w: unsupported sandbox provider", ErrInvalid)
	}
	return nil
}

// RuntimeRelease preserves the identities of one verified distribution. Docker
// may address the same archive by config ID or OCI manifest digest; microsandbox
// has its own imported OCI identity. These are not interchangeable hashes.
type RuntimeRelease struct {
	SourceCommit        string `json:"source_commit"`
	ImageID             string `json:"image_id"`
	ImageManifestDigest string `json:"image_manifest_digest"`
	MicrosandboxRef     string `json:"microsandbox_ref"`
	RuntimeSHA256       string `json:"runtime_sha256"`
	FirmwareSHA256      string `json:"firmware_sha256"`
}

func lowerHex(v string, bytes int) bool {
	x, err := hex.DecodeString(v)
	return err == nil && len(x) == bytes && hex.EncodeToString(x) == v
}

func (r RuntimeRelease) Validate() error {
	if !lowerHex(r.SourceCommit, 20) || !strings.HasPrefix(r.ImageID, "sha256:") || !lowerHex(strings.TrimPrefix(r.ImageID, "sha256:"), 32) ||
		!strings.HasPrefix(r.ImageManifestDigest, "sha256:") || !lowerHex(strings.TrimPrefix(r.ImageManifestDigest, "sha256:"), 32) ||
		!strings.HasPrefix(r.MicrosandboxRef, "oac-runtime@sha256:") || !lowerHex(strings.TrimPrefix(r.MicrosandboxRef, "oac-runtime@sha256:"), 32) ||
		!lowerHex(r.RuntimeSHA256, 32) || !lowerHex(r.FirmwareSHA256, 32) {
		return &ValidationError{Param: "runtime", Message: fmt.Sprintf("%s: Runtime must reference one immutable distribution", ErrInvalid)}
	}
	return nil
}

type DeploymentSpec struct {
	Resources Resources       `json:"resources"`
	Runtime   *RuntimeRelease `json:"runtime,omitempty"`
}

func (s DeploymentSpec) Validate(provider string) error {
	if err := s.Resources.Validate(provider); err != nil {
		return err
	}
	if provider == "e2b" {
		if s.Runtime != nil {
			return &ValidationError{Param: "runtime", Message: fmt.Sprintf("%s: E2B Runtime is selected by its immutable template build", ErrInvalid)}
		}
		return nil
	}
	if s.Runtime == nil {
		return &ValidationError{Param: "runtime", Message: fmt.Sprintf("%s: managed nodes require a pinned Runtime release", ErrInvalid)}
	}
	return s.Runtime.Validate()
}

func (s DeploymentSpec) Digest(provider string) string {
	raw, _ := json.Marshal(struct {
		Provider string `json:"provider"`
		DeploymentSpec
	}{provider, s})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
