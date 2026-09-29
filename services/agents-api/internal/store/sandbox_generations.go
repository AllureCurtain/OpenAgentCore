package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/db/sqlc"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/sandbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ClassifySandboxDeploymentChange resolves an omitted credential before validation.
// Explicit key submission is a verified mutation even when its bytes are unchanged.
func (s *Store) ClassifySandboxDeploymentChange(ctx context.Context, installation string, input SandboxDeploymentUpdateRequest) (SandboxDeploymentUpdateRequest, bool, error) {
	var unchanged bool
	err := s.resetTransaction(ctx, func(ctx context.Context, q *sqlc.Queries, d sqlc.RuntimeDeployment) error {
		if err := checkSandboxSwitch(ctx, q, d, installation, input); err != nil {
			return err
		}
		if input.E2B != nil {
			copy := *input.E2B
			copy.ReplaceCredential = copy.ReplaceCredential || copy.APIKey != ""
			input.E2B = &copy
			// Omitted endpoint selectors retain the committed E2B connection.
			if copy.APIURL == "" && copy.Domain == "" {
				input.E2B.APIURL, input.E2B.Domain = d.E2bApiUrl, d.E2bDomain
			}
			if copy.APIKey == "" && !copy.ReplaceCredential {
				key, err := s.credentialCipher.OpenSandboxDeployment(d.E2bCredential, installation, uint64(d.Generation))
				if err != nil {
					return ErrSandboxCredentialUnavailable
				}
				input.E2B.APIKey = string(key)
			}
			if input.E2B.Template == d.E2bTemplate && E2BResourcesPending(input.SandboxDeploymentSetupRequest) {
				var saved sandbox.DeploymentSpec
				if err := json.Unmarshal(d.Specification, &saved); err != nil {
					return err
				}
				input.Resources = saved.Resources
			}
		}
		if err := validateSandboxSelection(input.SandboxDeploymentSetupRequest); err != nil {
			return err
		}
		equal, err := s.sandboxSelectionEqual(d, input.SandboxDeploymentSetupRequest)
		unchanged = equal && (input.E2B == nil || !input.E2B.ReplaceCredential)
		return err
	})
	return input, unchanged, err
}

// GetSandboxAllocationSetup reads immutable ownership and the current credential
// in one snapshot. A released receipt remains historical but is never rebound.
func (s *Store) GetSandboxAllocationSetup(ctx context.Context, ref sandbox.Reference) (SandboxSetup, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return SandboxSetup{}, err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	lookup, err := deviceLookup(ref.TenantID, ref.EnvironmentID)
	if err != nil {
		return SandboxSetup{}, err
	}
	a, err := q.GetRuntimeAllocation(ctx, sqlc.GetRuntimeAllocationParams{TenantID: lookup.TenantID, EnvironmentID: lookup.ID})
	if err != nil {
		return SandboxSetup{}, err
	}
	if runtimeUUID(a.RuntimeAllocation.ID) != ref.AllocationID || !a.RuntimeAllocation.DeploymentGeneration.Valid || a.RuntimeAllocation.State == "released" {
		return SandboxSetup{}, ErrInvalidInput
	}
	d, err := q.GetRuntimeDeployment(ctx)
	if err != nil {
		return SandboxSetup{}, err
	}
	if a.RuntimeAllocation.ProviderKey != d.InstallationID {
		return SandboxSetup{}, sandbox.ErrOwnership
	}
	result, err := s.sandboxSetup(d)
	if err != nil {
		return SandboxSetup{}, err
	}
	generation := a.RuntimeAllocation.DeploymentGeneration.Int64
	if generation != d.Generation {
		g, err := q.GetSandboxGeneration(ctx, generation)
		if errors.Is(err, pgx.ErrNoRows) {
			return SandboxSetup{}, ErrSandboxDeploymentConflict
		}
		if err != nil {
			return SandboxSetup{}, err
		}
		if g.ProviderKind != d.ProviderKind {
			return SandboxSetup{}, ErrSandboxDeploymentConflict
		}
		result.Generation = uint64(generation)
		if err = json.Unmarshal(g.Specification, &result.Specification); err != nil {
			return SandboxSetup{}, err
		}
		if result.E2B != nil {
			result.E2B.Template = g.E2bTemplate
			result.E2B.APIURL, result.E2B.Domain = g.E2bApiUrl, g.E2bDomain
		}
	}
	return result, tx.Commit(ctx)
}

// SandboxGenerationPage is bounded; callers retain a deadline over the full scan.
func (s *Store) SandboxGenerationPage(ctx context.Context, after int64) ([]SandboxSetup, error) {
	rows, err := s.queries.ListSandboxGenerations(ctx, after)
	if err != nil {
		return nil, err
	}
	result := make([]SandboxSetup, 0, len(rows))
	for _, r := range rows {
		v := SandboxSetup{Generation: uint64(r.Generation), Provider: r.ProviderKind}
		if err := json.Unmarshal(r.Specification, &v.Specification); err != nil {
			return nil, err
		}
		if r.ProviderKind == "e2b" {
			v.E2B = &SandboxE2BConfiguration{Template: r.E2bTemplate, APIURL: r.E2bApiUrl, Domain: r.E2bDomain}
		}
		result = append(result, v)
	}
	return result, nil
}

// CollectSandboxGenerations shares the same serialization as admission and PUT.
func (s *Store) CollectSandboxGenerations(ctx context.Context) error {
	return s.resetTransaction(ctx, func(ctx context.Context, q *sqlc.Queries, _ sqlc.RuntimeDeployment) error {
		return q.CollectSandboxGenerations(ctx)
	})
}

// SandboxCredentialAllocationPage reads owned receipts without granting execution authority.
func (s *Store) SandboxCredentialAllocationPage(ctx context.Context, after string) ([]RuntimeAllocation, error) {
	id := pgtype.UUID{Valid: true}
	if after != "" {
		var err error
		id, err = parseID(after)
		if err != nil {
			return nil, err
		}
	}
	rows, err := s.queries.ListRuntimeAllocations(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]RuntimeAllocation, 0, len(rows))
	for _, r := range rows {
		out = append(out, runtimeAllocationFromRow(r.RuntimeAllocation, r.SessionID, r.TenantID, r.DeletedAt, r.Expired))
	}
	return out, nil
}
