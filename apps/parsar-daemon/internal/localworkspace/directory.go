package localworkspace

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"strings"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func (b *Binding) ListWorkspaceDirectory(ctx context.Context, path string, limit int) (agent.WorkspaceDirectoryResult, error) {
	if limit < 1 || limit > proto.WorkspaceDirectoryMaxEntries || len(path) > 4096 || strings.ContainsAny(path, "\\\x00\r\n") || (path != "" && (path == "." || !fs.ValidPath(path))) {
		return agent.WorkspaceDirectoryResult{}, agent.ErrWorkspaceReadInvalid
	}
	return b.listNativeDirectory(ctx, path, limit)
}

func decodeDirectory(data []byte, limit int) (agent.WorkspaceDirectoryResult, error) {
	var wire struct {
		Version   int                             `json:"version"`
		Error     *string                         `json:"error"`
		Directory *proto.WorkspaceDirectoryResult `json:"directory"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || decoder.Decode(new(any)) != io.EOF || wire.Version != 1 || (wire.Error == nil) == (wire.Directory == nil) {
		return agent.WorkspaceDirectoryResult{}, agent.ErrWorkspaceReadUncertain
	}
	if wire.Error != nil {
		err := agent.ErrWorkspaceReadUncertain
		switch *wire.Error {
		case "not_found":
			err = fs.ErrNotExist
		case "permission_denied":
			err = fs.ErrPermission
		case "invalid_path":
			err = agent.ErrWorkspaceReadInvalid
		case proto.WorkspaceReadNotDirectory:
			err = agent.ErrWorkspaceNotDirectory
		}
		return agent.WorkspaceDirectoryResult{}, err
	}
	if !proto.ValidWorkspaceDirectory(wire.Directory, limit) {
		return agent.WorkspaceDirectoryResult{}, agent.ErrWorkspaceReadUncertain
	}
	result := agent.WorkspaceDirectoryResult{Entries: make([]agent.WorkspaceDirectoryEntry, 0, len(wire.Directory.Entries)), Truncated: wire.Directory.Truncated}
	for _, e := range wire.Directory.Entries {
		result.Entries = append(result.Entries, agent.WorkspaceDirectoryEntry{Name: e.Name, Kind: e.Kind, SizeBytes: e.SizeBytes})
	}
	return result, nil
}
