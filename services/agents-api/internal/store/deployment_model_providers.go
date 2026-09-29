package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	v1 "github.com/MiniMax-AI-Dev/parsar/contracts/agents-api/v1"
	"github.com/MiniMax-AI-Dev/parsar/services/agents-api/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrModelProviderRequired reports a hosted or self-hosted Session that has no
// frozen model provider and therefore cannot run.
var ErrModelProviderRequired = errors.New("the Session has no model provider")

// DeploymentModelProvider is the safe view of one harness's deployment default.
// The API key is write-only; only its presence is reported.
type DeploymentModelProvider struct {
	Harness       string
	Provider      v1.ModelProviderView
	UpdatedAt     time.Time
	LastUsedAt    *time.Time
	LastErrorCode *string
	LastErrorAt   *time.Time
}

func deploymentModelProvider(harness, protocol, baseURL string, contextWindow, maxOutputTokens int32, updatedAt time.Time, lastUsedAt, lastErrorAt pgtype.Timestamptz, lastErrorCode pgtype.Text) DeploymentModelProvider {
	var code *string
	if lastErrorCode.Valid {
		code = &lastErrorCode.String
	}
	return DeploymentModelProvider{Harness: harness, UpdatedAt: updatedAt, LastUsedAt: resetTimestamp(lastUsedAt), LastErrorAt: resetTimestamp(lastErrorAt), LastErrorCode: code, Provider: v1.ModelProviderView{
		Protocol: protocol, BaseURL: baseURL, ContextWindow: contextWindow, MaxOutputTokens: maxOutputTokens, APIKeyConfigured: true,
	}}
}

// ListDeploymentModelProviders reads the configured defaults without decryption.
func (s *Store) ListDeploymentModelProviders(ctx context.Context) ([]DeploymentModelProvider, error) {
	rows, err := s.queries.ListDeploymentModelProviders(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]DeploymentModelProvider, 0, len(rows))
	for _, row := range rows {
		result = append(result, deploymentModelProvider(row.Harness, row.Protocol, row.BaseUrl, row.ContextWindow, row.MaxOutputTokens, row.UpdatedAt.Time, row.LastUsedAt, row.LastErrorAt, row.LastErrorCode))
	}
	return result, nil
}

// SetDeploymentModelProvider replaces a harness's complete default bundle and
// audits the write in the same transaction, without the key.
func (s *Store) SetDeploymentModelProvider(ctx context.Context, harness string, provider v1.ModelProviderInput) (DeploymentModelProvider, error) {
	if err := provider.ValidateHarness(harness); err != nil {
		return DeploymentModelProvider{}, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}
	raw, err := json.Marshal(provider)
	if err != nil {
		return DeploymentModelProvider{}, err
	}
	encrypted, err := s.credentialCipher.SealDeploymentModelProvider(raw, harness)
	if err != nil {
		return DeploymentModelProvider{}, ErrCredentialStorageUnavailable
	}
	var result DeploymentModelProvider
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		row, err := q.UpsertDeploymentModelProvider(ctx, sqlc.UpsertDeploymentModelProviderParams{
			Harness: harness, Protocol: provider.Protocol, BaseUrl: provider.BaseURL,
			ContextWindow: provider.ContextWindow, MaxOutputTokens: provider.MaxOutputTokens, EncryptedConfig: encrypted, Revision: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		})
		if err != nil {
			return err
		}
		result = deploymentModelProvider(row.Harness, row.Protocol, row.BaseUrl, row.ContextWindow, row.MaxOutputTokens, row.UpdatedAt.Time, row.LastUsedAt, row.LastErrorAt, row.LastErrorCode)
		return recordDeploymentMutation(ctx, q, "set", "deployment_model_provider", harness)
	})
	return result, err
}

// DeleteDeploymentModelProvider is idempotent; each successful call is audited.
// Sessions that already froze the default keep their snapshot.
func (s *Store) DeleteDeploymentModelProvider(ctx context.Context, harness string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		if _, err := q.DeleteDeploymentModelProvider(ctx, harness); err != nil {
			return err
		}
		return recordDeploymentMutation(ctx, q, "delete", "deployment_model_provider", harness)
	})
}

// DeploymentModelProvider decrypts a harness's default for Session creation. It
// returns nil when none is configured and fails closed when the bundle cannot
// be decrypted.
func (s *Store) DeploymentModelProvider(ctx context.Context, harness string) (*DeploymentModelProviderSnapshot, error) {
	snapshot, err := s.queries.GetDeploymentModelProviderSecret(ctx, harness)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := s.credentialCipher.OpenDeploymentModelProvider(snapshot.EncryptedConfig, harness)
	if err != nil {
		return nil, ErrCredentialStorageUnavailable
	}
	var provider v1.ModelProviderInput
	if json.Unmarshal(raw, &provider) != nil || provider.ValidateHarness(harness) != nil {
		return nil, ErrCredentialStorageUnavailable
	}
	return &DeploymentModelProviderSnapshot{Provider: &provider, Revision: uuid.UUID(snapshot.Revision.Bytes)}, nil
}

// DeploymentModelProviderSnapshot pairs one decrypted bundle with its private
// revision from the same database read. It never enters a public projection.
type DeploymentModelProviderSnapshot struct {
	Provider *v1.ModelProviderInput
	Revision uuid.UUID `json:"-"`
}
