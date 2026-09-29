package sandbox

import (
	"errors"
	"testing"
)

func TestDeploymentValidationFieldsPreserveMessages(t *testing.T) {
	for _, tc := range []struct {
		provider       string
		resources      Resources
		param, message string
		min, max       *uint32
	}{
		{"docker", Resources{}, "resources.cpus", "cpus must be 1..255 and memory_mib must be 512..1048576", validationBound(1), validationBound(255)},
		{"docker", Resources{CPUs: 1, MemoryMiB: 511}, "resources.memory_mib", "cpus must be 1..255 and memory_mib must be 512..1048576", validationBound(512), validationBound(1048576)},
		{"microsandbox", Resources{CPUs: 1, MemoryMiB: 512}, "resources.root_disk_mib", "microsandbox requires root_disk_mib and environment_disk_mib of at least 1024 MiB", validationBound(1024), nil},
		{"microsandbox", Resources{CPUs: 1, MemoryMiB: 512, RootDiskMiB: 1024}, "resources.environment_disk_mib", "microsandbox requires root_disk_mib and environment_disk_mib of at least 1024 MiB", validationBound(1024), nil},
		{"docker", Resources{CPUs: 1, MemoryMiB: 512, RootDiskMiB: 1}, "resources.root_disk_mib", "docker does not support independent disk capacity limits", validationBound(0), validationBound(0)},
		{"e2b", Resources{CPUs: 1, MemoryMiB: 512, EnvironmentDiskMiB: 1}, "resources.environment_disk_mib", "e2b does not support independent disk capacity limits", validationBound(0), validationBound(0)},
	} {
		err := tc.resources.Validate(tc.provider)
		var field *ValidationError
		if !errors.As(err, &field) || !errors.Is(err, ErrInvalid) || err.Error() != ErrInvalid.Error()+": "+tc.message || field.Param != tc.param {
			t.Fatalf("wrong error: %#v", err)
		}
		for i, bound := range []*uint32{field.Min, field.Max} {
			want := []*uint32{tc.min, tc.max}[i]
			if (bound == nil) != (want == nil) || (bound != nil && *bound != *want) {
				t.Fatal("wrong fixed bounds", field)
			}
		}
	}
	for _, tc := range []struct {
		provider string
		runtime  *RuntimeRelease
		message  string
	}{
		{"docker", nil, "managed nodes require a pinned Runtime release"},
		{"docker", &RuntimeRelease{}, "Runtime must reference one immutable distribution"},
		{"e2b", &RuntimeRelease{}, "E2B Runtime is selected by its immutable template build"},
	} {
		err := (DeploymentSpec{Resources: Resources{CPUs: 1, MemoryMiB: 512}, Runtime: tc.runtime}).Validate(tc.provider)
		var field *ValidationError
		if !errors.As(err, &field) || field.Param != "runtime" || err.Error() != ErrInvalid.Error()+": "+tc.message || !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if err := (Resources{CPUs: 255, MemoryMiB: 1048576}).Validate("docker"); err != nil {
		t.Fatal(err)
	}
	var field *ValidationError
	err := (Resources{CPUs: 1, MemoryMiB: 512}).Validate("private-provider")
	if errors.As(err, &field) || err.Error() != ErrInvalid.Error()+": unsupported sandbox provider" {
		t.Fatal("unknown provider reclassified", err)
	}
}
