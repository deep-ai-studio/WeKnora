package service

import (
	"context"
	"strings"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
)

type preparedOpenAPIChat struct {
	chatCtx           context.Context
	session           *types.Session
	sessionMapping    *types.OpenAPISessionMapping
	kbID              string
	query             string
	mode              string
	tenantID          uint64
	externalSessionID string
}

func (s *openAPIService) prepareOpenAPIChat(
	ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest,
) (*preparedOpenAPIChat, error) {
	externalUserID := strings.TrimSpace(req.ExternalUserID)
	kbID := strings.TrimSpace(req.KnowledgeBaseID)
	query := strings.TrimSpace(req.Query)
	if externalUserID == "" {
		return nil, errors.NewBadRequestError("external_user_id is required")
	}
	if kbID == "" {
		return nil, errors.NewBadRequestError("knowledge_base_id is required")
	}
	if query == "" {
		return nil, errors.NewBadRequestError("query is required")
	}
	if !isKBAllowed([]string(client.AllowedKBIDs), kbID) {
		return nil, errors.NewForbiddenError("knowledge base is not allowed for this client")
	}

	mode, err := resolveOpenAPIChatMode(req.Mode)
	if err != nil {
		return nil, err
	}

	tenant, err := s.tenantService.GetTenantByID(ctx, client.TenantID)
	if err != nil || tenant == nil {
		return nil, errors.NewUnauthorizedError("invalid tenant for Open API client")
	}

	internalUserID, err := s.ensureUserMapping(ctx, client, externalUserID)
	if err != nil {
		return nil, err
	}

	chatCtx := withOpenAPIChatContext(ctx, tenant, internalUserID)

	kb, err := s.knowledgeBaseService.GetKnowledgeBaseByID(chatCtx, kbID)
	if err != nil || kb == nil {
		return nil, errors.NewNotFoundError("knowledge base not found")
	}
	if mode == types.OpenAPIChatModeWikiQA && !kb.Capabilities().Wiki {
		return nil, errors.NewBadRequestError("wiki-qa mode requires a wiki-enabled knowledge base")
	}

	session, sessionMapping, err := s.resolveSession(
		chatCtx, client, externalUserID, req.SessionID, req.ExternalSessionID, kbID,
	)
	if err != nil {
		return nil, err
	}

	externalSessionID := strings.TrimSpace(req.ExternalSessionID)
	if externalSessionID == "" && sessionMapping != nil {
		externalSessionID = sessionMapping.ExternalSessionID
	}

	return &preparedOpenAPIChat{
		chatCtx:           chatCtx,
		session:           session,
		sessionMapping:    sessionMapping,
		kbID:              kbID,
		query:             query,
		mode:              mode,
		tenantID:          client.TenantID,
		externalSessionID: externalSessionID,
	}, nil
}

func buildOpenAPIQARequest(
	ctx context.Context, prepared *preparedOpenAPIChat, userMsgID, assistantMsgID string,
) (*types.QARequest, error) {
	qaReq := &types.QARequest{
		Session:            prepared.session,
		Query:              prepared.query,
		AssistantMessageID: assistantMsgID,
		UserMessageID:      userMsgID,
		KnowledgeBaseIDs:   []string{prepared.kbID},
	}
	if prepared.mode == types.OpenAPIChatModeWikiQA {
		agent := types.GetBuiltinAgentWithContext(ctx, types.BuiltinWikiResearcherID, prepared.tenantID)
		if agent == nil {
			return nil, errors.NewInternalServerError("wiki-qa agent not available")
		}
		qaReq.CustomAgent = agent
	}
	return qaReq, nil
}
