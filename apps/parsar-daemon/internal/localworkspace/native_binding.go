package localworkspace

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/MiniMax-AI-Dev/parsar/internal/runtimefs"
	"github.com/google/uuid"
)

// ExecutionMode selects the installation layout, never a permission boundary.
func (b *Binding) ExecutionMode() string {
	if b == nil {
		return ""
	}
	return b.executionMode
}

func newNativeBinding(environment, session, workspace, capabilities string) (*Binding, error) {
	for _, value := range []string{environment, session} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return nil, errors.New("local workspace requires canonical resource identities")
		}
	}
	for _, path := range []string{workspace, capabilities} {
		if runtimefs.ValidateLocalPath(path) != nil || filepath.Dir(path) == path {
			return nil, errors.New("local workspace requires absolute installation paths")
		}
	}
	info, err := os.Stat(workspace)
	if err != nil || !info.IsDir() {
		return nil, errors.New("local workspace root must be an existing directory")
	}
	return &Binding{environment: environment, stateKey: "agents-api-" + session, workspace: workspace, capabilityRoot: capabilities, executionMode: "native", writer: &fileWriter{}}, nil
}

// ReadToolEnvironment reads explicit initialization values for this installation.
func (b *Binding) ReadToolEnvironment() (map[string]string, error) {
	return ReadOptionalToolEnvironment()
}
