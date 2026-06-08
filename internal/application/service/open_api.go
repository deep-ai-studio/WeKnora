package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

type openAPIService struct {
	repo                 interfaces.OpenAPIRepository
	tenantService        interfaces.TenantService
	sessionService       interfaces.SessionService
	messageService       interfaces.MessageService
	knowledgeBaseService interfaces.KnowledgeBaseService
}

// NewOpenAPIService wires the partner Open API service.
func NewOpenAPIService(
	repo interfaces.OpenAPIRepository,
	tenantService interfaces.TenantService,
	sessionService interfaces.SessionService,
	messageService interfaces.MessageService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
) interfaces.OpenAPIService {
	return &openAPIService{
		repo:                 repo,
		tenantService:        tenantService,
		sessionService:       sessionService,
		messageService:       messageService,
		knowledgeBaseService: knowledgeBaseService,
	}
}

func (s *openAPIService) ValidateClientByAPIKey(ctx context.Context, apiKey string) (*types.OpenAPIClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || !strings.HasPrefix(apiKey, openAPIKeyPrefix) {
		return nil, errors.NewUnauthorizedError("invalid Open API key")
	}
	client, err := s.repo.GetActiveClientByKeyHash(ctx, HashOpenAPIKey(apiKey))
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, errors.NewUnauthorizedError("invalid Open API key")
	}
	return client, nil
}

func (s *openAPIService) CreateClient(
	ctx context.Context, req *types.CreateOpenAPIClientRequest,
) (*types.CreateOpenAPIClientResponse, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, errors.NewUnauthorizedError("tenant context missing")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.NewBadRequestError("name is required")
	}
	if len(req.AllowedKBIDs) == 0 {
		return nil, errors.NewBadRequestError("allowed_kb_ids must not be empty")
	}

	plaintext, hash, err := GenerateOpenAPIKey()
	if err != nil {
		return nil, errors.NewInternalServerError(err.Error())
	}

	client := &types.OpenAPIClient{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		Name:         name,
		APIKeyHash:   hash,
		AllowedKBIDs: types.StringArray(req.AllowedKBIDs),
		Status:       types.OpenAPIClientStatusActive,
	}
	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, errors.NewInternalServerError(err.Error())
	}
	return &types.CreateOpenAPIClientResponse{
		Client: client,
		APIKey: plaintext,
	}, nil
}

func (s *openAPIService) ListClients(ctx context.Context) ([]*types.OpenAPIClient, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, errors.NewUnauthorizedError("tenant context missing")
	}
	return s.repo.ListClientsByTenant(ctx, tenantID)
}

func (s *openAPIService) RevokeClient(ctx context.Context, clientID string) error {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return errors.NewUnauthorizedError("tenant context missing")
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return errors.NewBadRequestError("client id is required")
	}
	client, err := s.repo.GetClientByID(ctx, tenantID, clientID)
	if err != nil {
		return errors.NewInternalServerError(err.Error())
	}
	if client == nil {
		return errors.NewNotFoundError("Open API client not found")
	}
	if client.Status == types.OpenAPIClientStatusRevoked {
		return nil
	}
	return s.repo.RevokeClient(ctx, tenantID, clientID)
}

func (s *openAPIService) Chat(
	ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest,
) (*types.OpenAPIChatResponse, error) {
	if req.Stream {
		return nil, errors.NewBadRequestError("stream=true requires Accept: text/event-stream; use streaming chat handler")
	}

	prepared, err := s.prepareOpenAPIChat(ctx, client, req)
	if err != nil {
		return nil, err
	}

	answer, isFallback, references, err := s.runQA(
		prepared.chatCtx, prepared.session, prepared.kbID, prepared.query, prepared.mode, prepared.tenantID,
	)
	if err != nil {
		return nil, err
	}

	resp := &types.OpenAPIChatResponse{
		SessionID:         prepared.session.ID,
		ExternalSessionID: prepared.externalSessionID,
		Answer:            answer,
		IsFallback:        isFallback,
		References:        references,
	}
	return resp, nil
}

func (s *openAPIService) ensureUserMapping(
	ctx context.Context, client *types.OpenAPIClient, externalUserID string,
) (string, error) {
	mapping, err := s.repo.GetUserMapping(ctx, client.TenantID, client.ID, externalUserID)
	if err != nil {
		return "", errors.NewInternalServerError(err.Error())
	}
	if mapping != nil {
		_ = s.repo.TouchUserMapping(ctx, mapping.ID)
		return mapping.InternalUserID, nil
	}

	internalUserID := BuildOpenAPIInternalUserID(client.ID, externalUserID)
	newMapping := &types.OpenAPIUserMapping{
		TenantID:       client.TenantID,
		ClientID:       client.ID,
		ExternalUserID: externalUserID,
		InternalUserID: internalUserID,
	}
	if err := s.repo.CreateUserMapping(ctx, newMapping); err != nil {
		return "", errors.NewInternalServerError(err.Error())
	}
	logger.Infof(ctx, "[open-api] created user mapping client=%s external_user=%s internal_user=%s",
		client.ID, externalUserID, internalUserID)
	return internalUserID, nil
}

func (s *openAPIService) resolveSession(
	ctx context.Context,
	client *types.OpenAPIClient,
	externalUserID, sessionID, externalSessionID, kbID string,
) (*types.Session, *types.OpenAPISessionMapping, error) {
	sessionID = strings.TrimSpace(sessionID)
	externalSessionID = strings.TrimSpace(externalSessionID)

	if sessionID != "" {
		session, err := s.sessionService.GetSession(ctx, sessionID)
		if err != nil {
			if stderrors.Is(err, errors.ErrSessionNotFound) {
				return nil, nil, errors.NewNotFoundError("session not found")
			}
			return nil, nil, errors.NewInternalServerError(err.Error())
		}
		return session, nil, nil
	}

	if externalSessionID != "" {
		mapping, err := s.repo.GetSessionMapping(ctx, client.TenantID, client.ID, externalUserID, externalSessionID)
		if err != nil {
			return nil, nil, errors.NewInternalServerError(err.Error())
		}
		if mapping != nil {
			session, err := s.sessionService.GetSession(ctx, mapping.InternalSessionID)
			if err == nil && session != nil {
				_ = s.repo.TouchSessionMapping(ctx, mapping.ID)
				return session, mapping, nil
			}
		}
	}

	title := "Open API"
	if externalSessionID != "" {
		title = fmt.Sprintf("Open API %s", externalSessionID)
	}
	newSession := &types.Session{
		TenantID:    client.TenantID,
		Title:       title,
		Description: "Created via Open API",
	}
	if userID, ok := types.UserIDFromContext(ctx); ok {
		newSession.UserID = userID
	}
	session, err := s.sessionService.CreateSession(ctx, newSession)
	if err != nil {
		return nil, nil, errors.NewInternalServerError(err.Error())
	}

	var mapping *types.OpenAPISessionMapping
	if externalSessionID != "" {
		mapping = &types.OpenAPISessionMapping{
			TenantID:          client.TenantID,
			ClientID:          client.ID,
			ExternalUserID:    externalUserID,
			ExternalSessionID: externalSessionID,
			InternalSessionID: session.ID,
			KnowledgeBaseID:   kbID,
		}
		if err := s.repo.CreateSessionMapping(ctx, mapping); err != nil {
			return nil, nil, errors.NewInternalServerError(err.Error())
		}
	}
	return session, mapping, nil
}

func resolveOpenAPIChatMode(mode string) (string, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return types.OpenAPIChatModeWikiQA, nil
	}
	switch mode {
	case types.OpenAPIChatModeWikiQA, types.OpenAPIChatModeRAGQA:
		return mode, nil
	default:
		return "", errors.NewBadRequestError("mode must be wiki-qa or rag-qa")
	}
}

func withOpenAPIChatContext(ctx context.Context, tenant *types.Tenant, internalUserID string) context.Context {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant.ID)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, internalUserID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
	return ctx
}
