package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
)

// OpenAPIRepository persists partner credentials and identity mappings.
type OpenAPIRepository interface {
	CreateClient(ctx context.Context, client *types.OpenAPIClient) error
	GetActiveClientByKeyHash(ctx context.Context, keyHash string) (*types.OpenAPIClient, error)
	ListClientsByTenant(ctx context.Context, tenantID uint64) ([]*types.OpenAPIClient, error)
	GetClientByID(ctx context.Context, tenantID uint64, clientID string) (*types.OpenAPIClient, error)
	RevokeClient(ctx context.Context, tenantID uint64, clientID string) error

	GetUserMapping(ctx context.Context, tenantID uint64, clientID, externalUserID string) (*types.OpenAPIUserMapping, error)
	CreateUserMapping(ctx context.Context, mapping *types.OpenAPIUserMapping) error
	TouchUserMapping(ctx context.Context, id uint64) error

	GetSessionMapping(ctx context.Context, tenantID uint64, clientID, externalUserID, externalSessionID string) (*types.OpenAPISessionMapping, error)
	CreateSessionMapping(ctx context.Context, mapping *types.OpenAPISessionMapping) error
	TouchSessionMapping(ctx context.Context, id uint64) error
}

// OpenAPIChatStreamJob identifies a streaming chat turn prepared by the service.
type OpenAPIChatStreamJob struct {
	ChatCtx            context.Context
	Session            *types.Session
	AssistantMessage   *types.Message
	UserMessageID      string
	RequestID          string
	ExternalSessionID  string
	Mode               string
	KnowledgeBaseID    string
	TenantID           uint64
	Query              string
}

// OpenAPIService implements partner Open API business logic.
type OpenAPIService interface {
	ValidateClientByAPIKey(ctx context.Context, apiKey string) (*types.OpenAPIClient, error)
	CreateClient(ctx context.Context, req *types.CreateOpenAPIClientRequest) (*types.CreateOpenAPIClientResponse, error)
	ListClients(ctx context.Context) ([]*types.OpenAPIClient, error)
	RevokeClient(ctx context.Context, clientID string) error
	Chat(ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest) (*types.OpenAPIChatResponse, error)
	PrepareChatStream(ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest) (*OpenAPIChatStreamJob, error)
	RunStreamQA(ctx context.Context, job *OpenAPIChatStreamJob, eventBus *event.EventBus) error
}
