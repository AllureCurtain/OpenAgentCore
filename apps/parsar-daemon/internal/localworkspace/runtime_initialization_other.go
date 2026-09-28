//go:build !linux

package localworkspace

import (
	"context"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
)

func (b *Binding) initializeRuntime(context.Context, proto.RuntimeInitialization) error {
	return agentcapabilities.ErrInvalid
}
func (b *Binding) installInitialFile(context.Context, proto.RuntimeInitialFile, []byte) error {
	return agentcapabilities.ErrInvalid
}
