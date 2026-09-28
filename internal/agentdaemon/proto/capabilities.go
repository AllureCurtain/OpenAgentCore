package proto

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentcapabilities"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentplugin"
	"github.com/MiniMax-AI-Dev/parsar/internal/agentskill"
	"github.com/google/uuid"
)

const (
	TypeCapabilitiesPrepare   = "capabilities_prepare"
	TypeCapabilitiesResult    = "capabilities_result"
	CapabilitiesMaxBytes      = 50 << 20
	CapabilitiesChunkBytes    = WorkspaceWriteChunkBytes
	CapabilitiesMaxFrameBytes = 1 << 20
)

// CapabilitiesPreparePayload imports an inert archive or finalizes frozen source
// selections. Runtime owns all destinations; this protocol supplies no command or
// installation path. Envelope.ID identifies one connection-local transfer.
type CapabilitiesPreparePayload struct {
	Step          string                   `json:"step"`
	EnvironmentID string                   `json:"environment_id,omitempty"`
	SessionID     string                   `json:"session_id,omitempty"`
	Action        string                   `json:"action,omitempty"`
	Slot          int                      `json:"slot,omitempty"`
	Skill         *agentskill.Metadata     `json:"skill,omitempty"`
	Plugin        *agentplugin.Metadata    `json:"plugin,omitempty"`
	Sources       *agentcapabilities.Input `json:"sources,omitempty"`
	SizeBytes     int                      `json:"size_bytes,omitempty"`
	SHA256        string                   `json:"sha256,omitempty"`
	Offset        int                      `json:"offset,omitempty"`
	Data          []byte                   `json:"data,omitempty"`
}

type CapabilitiesResultPayload struct {
	Outcome   string `json:"outcome"`
	Offset    int    `json:"offset,omitempty"`
	SizeBytes int    `json:"size_bytes,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

func ValidCapabilitiesPrepareRequest(p CapabilitiesPreparePayload) bool {
	if p.Step == "begin" {
		for _, id := range []string{p.EnvironmentID, p.SessionID} {
			value, err := uuid.Parse(id)
			if err != nil || value == uuid.Nil || value.String() != id {
				return false
			}
		}
		if p.Offset != 0 || len(p.Data) != 0 {
			return false
		}
		switch p.Action {
		case "skill":
			if p.Skill == nil || p.Plugin != nil || p.Sources != nil || p.Slot != 0 ||
				agentcapabilities.ValidateInput(agentcapabilities.Input{Skills: []agentskill.Metadata{*p.Skill}}) != nil {
				return false
			}
		case "plugin":
			if p.Plugin == nil || p.Skill != nil || p.Sources != nil || p.Slot < 0 || p.Slot >= 50 ||
				agentcapabilities.ValidateInput(agentcapabilities.Input{Plugins: []agentplugin.Metadata{*p.Plugin}}) != nil {
				return false
			}
		case "finalize":
			if p.Skill != nil || p.Plugin != nil || p.Sources == nil || p.Slot != 0 ||
				p.SizeBytes != 0 || p.SHA256 != "" || agentcapabilities.ValidateInput(*p.Sources) != nil {
				return false
			}
		default:
			return false
		}
		if p.Action != "finalize" {
			digest, err := hex.DecodeString(p.SHA256)
			if err != nil || len(digest) != 32 || strings.ToLower(p.SHA256) != p.SHA256 ||
				p.SizeBytes <= 0 || p.SizeBytes > CapabilitiesMaxBytes {
				return false
			}
		}
		encoded, err := json.Marshal(p)
		return err == nil && len(encoded) <= CapabilitiesMaxFrameBytes
	}
	if p.EnvironmentID != "" || p.SessionID != "" || p.Action != "" || p.Slot != 0 ||
		p.Skill != nil || p.Plugin != nil || p.Sources != nil || p.SizeBytes != 0 || p.SHA256 != "" {
		return false
	}
	switch p.Step {
	case "chunk":
		return p.Offset >= 0 && p.Offset <= CapabilitiesMaxBytes && len(p.Data) > 0 &&
			len(p.Data) <= CapabilitiesChunkBytes && len(p.Data) <= CapabilitiesMaxBytes-p.Offset
	case "commit":
		return p.Offset == 0 && len(p.Data) == 0
	default:
		return false
	}
}

// ValidCapabilitiesResult accepts only the expected success receipt or a finite
// terminal category. An unknown effect can never be represented as rejection.
func ValidCapabilitiesResult(r CapabilitiesResultPayload, expected string, offset, size int) bool {
	switch r.Outcome {
	case "rejected":
		if r.Offset != 0 || r.SizeBytes != 0 {
			return false
		}
		switch r.ErrorCode {
		case "invalid_request", "resource_unavailable", "capabilities_capacity", "capabilities_unsupported", "capabilities_rejected":
			return true
		default:
			return false
		}
	case "failed":
		return r.Offset == 0 && r.SizeBytes == 0 && r.ErrorCode == "capabilities_failed"
	case "unknown":
		return r.Offset == 0 && r.SizeBytes == 0 && r.ErrorCode == "capabilities_unconfirmed"
	}
	if r.Outcome != expected || r.Offset != offset || r.ErrorCode != "" {
		return false
	}
	switch expected {
	case "completed":
		return r.SizeBytes == size
	case "ready", "received":
		return r.SizeBytes == 0
	default:
		return false
	}
}
