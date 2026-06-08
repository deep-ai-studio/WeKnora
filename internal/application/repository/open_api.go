package repository

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type openAPIRepository struct {
	db *gorm.DB
}

// NewOpenAPIRepository constructs the Open API persistence layer.
func NewOpenAPIRepository(db *gorm.DB) interfaces.OpenAPIRepository {
	return &openAPIRepository{db: db}
}

func (r *openAPIRepository) CreateClient(ctx context.Context, client *types.OpenAPIClient) error {
	now := time.Now()
	client.CreatedAt = now
	client.UpdatedAt = now
	return r.db.WithContext(ctx).Create(client).Error
}

func (r *openAPIRepository) GetActiveClientByKeyHash(ctx context.Context, keyHash string) (*types.OpenAPIClient, error) {
	var client types.OpenAPIClient
	err := r.db.WithContext(ctx).
		Where("api_key_hash = ? AND status = ?", keyHash, types.OpenAPIClientStatusActive).
		First(&client).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}

func (r *openAPIRepository) ListClientsByTenant(ctx context.Context, tenantID uint64) ([]*types.OpenAPIClient, error) {
	var clients []*types.OpenAPIClient
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&clients).Error
	return clients, err
}

func (r *openAPIRepository) GetClientByID(ctx context.Context, tenantID uint64, clientID string) (*types.OpenAPIClient, error) {
	var client types.OpenAPIClient
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, clientID).
		First(&client).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}

func (r *openAPIRepository) RevokeClient(ctx context.Context, tenantID uint64, clientID string) error {
	return r.db.WithContext(ctx).
		Model(&types.OpenAPIClient{}).
		Where("tenant_id = ? AND id = ?", tenantID, clientID).
		Updates(map[string]interface{}{
			"status":     types.OpenAPIClientStatusRevoked,
			"updated_at": time.Now(),
		}).Error
}

func (r *openAPIRepository) GetUserMapping(
	ctx context.Context, tenantID uint64, clientID, externalUserID string,
) (*types.OpenAPIUserMapping, error) {
	var mapping types.OpenAPIUserMapping
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND client_id = ? AND external_user_id = ?", tenantID, clientID, externalUserID).
		First(&mapping).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &mapping, nil
}

func (r *openAPIRepository) CreateUserMapping(ctx context.Context, mapping *types.OpenAPIUserMapping) error {
	now := time.Now()
	mapping.FirstSeenAt = now
	mapping.LastSeenAt = now
	return r.db.WithContext(ctx).Create(mapping).Error
}

func (r *openAPIRepository) TouchUserMapping(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&types.OpenAPIUserMapping{}).
		Where("id = ?", id).
		Update("last_seen_at", time.Now()).Error
}

func (r *openAPIRepository) GetSessionMapping(
	ctx context.Context, tenantID uint64, clientID, externalUserID, externalSessionID string,
) (*types.OpenAPISessionMapping, error) {
	var mapping types.OpenAPISessionMapping
	err := r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND client_id = ? AND external_user_id = ? AND external_session_id = ?",
			tenantID, clientID, externalUserID, externalSessionID,
		).
		First(&mapping).Error
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &mapping, nil
}

func (r *openAPIRepository) CreateSessionMapping(ctx context.Context, mapping *types.OpenAPISessionMapping) error {
	now := time.Now()
	mapping.CreatedAt = now
	mapping.LastActiveAt = now
	return r.db.WithContext(ctx).Create(mapping).Error
}

func (r *openAPIRepository) TouchSessionMapping(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).
		Model(&types.OpenAPISessionMapping{}).
		Where("id = ?", id).
		Update("last_active_at", time.Now()).Error
}
