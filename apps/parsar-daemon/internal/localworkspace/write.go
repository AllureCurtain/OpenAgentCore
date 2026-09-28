package localworkspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"strings"

	"github.com/MiniMax-AI-Dev/parsar/apps/parsar-daemon/internal/agent"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

// This private bound matches the existing installer; it is not an upstream limit.
const WriteMaxBytes = proto.WorkspaceWriteMaxBytes

var _ agent.WorkspaceWriter = (*Binding)(nil)

func (b *Binding) AcceptsFileWrite(environment, session string) bool {
	return b != nil && b.writer != nil && b.environment == environment && b.stateKey == "agents-api-"+session
}

// WriteWorkspaceFile starts only after the caller supplies the complete bounded
// body. Core must persist mutation ownership before invoking this operation.
func (b *Binding) WriteWorkspaceFile(ctx context.Context, path string, data []byte) (result agent.WorkspaceWriteResult, err error) {
	if b == nil || b.writer == nil {
		return result, agent.ErrWorkspaceWriteUnsupported
	}
	if len(data) > WriteMaxBytes || len(path) > 4096 || path == "." || !fs.ValidPath(path) || strings.ContainsAny(path, "\\\x00\r\n") {
		return result, agent.ErrWorkspaceWriteInvalid
	}
	if ctx.Err() != nil {
		return result, agent.ErrWorkspaceWriteUnavailable
	}
	w := b.writer
	if !w.mu.TryLock() {
		return result, agent.ErrWorkspaceWriteBusy
	}
	defer w.mu.Unlock()
	if w.uncertain {
		return result, agent.ErrWorkspaceWriteUncertain
	}
	defer func() { w.uncertain = errors.Is(err, agent.ErrWorkspaceWriteUncertain) }()
	// Once admitted, finish this synchronous mutation before returning ownership.
	return b.writeNativeFile(context.WithoutCancel(ctx), path, data)
}

// A malformed helper cannot grow the daemon's output buffer without bound.
type writeOutput struct{ data bytes.Buffer }

func (b *writeOutput) Bytes() []byte { return b.data.Bytes() }

func (b *writeOutput) Write(p []byte) (int, error) {
	if len(p) > 1024-b.data.Len() {
		return 0, errors.New("local write response exceeds limit")
	}
	return b.data.Write(p)
}

func decodeWrite(data []byte, expected int) (agent.WorkspaceWriteResult, error) {
	var wire struct {
		Version   int     `json:"version"`
		Outcome   string  `json:"outcome"`
		SizeBytes *int64  `json:"size_bytes"`
		Error     *string `json:"error"`
	}
	invalid := agent.ErrWorkspaceWriteUncertain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || decoder.Decode(new(any)) != io.EOF || wire.Version != 1 {
		return agent.WorkspaceWriteResult{}, invalid
	}
	if wire.Outcome == "completed" && wire.Error == nil && wire.SizeBytes != nil && *wire.SizeBytes == int64(expected) {
		return agent.WorkspaceWriteResult{SizeBytes: *wire.SizeBytes}, nil
	}
	if wire.Outcome == "failed" && wire.SizeBytes == nil && wire.Error != nil {
		switch *wire.Error {
		case "invalid_input", "write_failed":
			return agent.WorkspaceWriteResult{}, agent.ErrWorkspaceWriteRejected
		case "destination_directory":
			return agent.WorkspaceWriteResult{}, agent.ErrWorkspaceWriteDirectory
		case "unsafe_destination":
			return agent.WorkspaceWriteResult{}, agent.ErrWorkspaceWriteUnsafe
		}
	}
	return agent.WorkspaceWriteResult{}, invalid
}
