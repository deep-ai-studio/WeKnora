package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

func (s *openAPIService) PrepareChatStream(
	ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest,
) (*interfaces.OpenAPIChatStreamJob, error) {
	prepared, err := s.prepareOpenAPIChat(ctx, client, req)
	if err != nil {
		return nil, err
	}

	requestID := uuid.NewString()
	now := time.Now()

	userMsg, err := s.messageService.CreateMessage(ctx, &types.Message{
		SessionID:   prepared.session.ID,
		Role:        "user",
		Content:     prepared.query,
		RequestID:   requestID,
		CreatedAt:   now,
		IsCompleted: true,
		Channel:     types.ChannelAPI,
	})
	if err != nil {
		return nil, errors.NewInternalServerError(fmt.Sprintf("create user message: %v", err))
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, &types.Message{
		SessionID:   prepared.session.ID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   now,
		IsCompleted: false,
		Channel:     types.ChannelAPI,
	})
	if err != nil {
		return nil, errors.NewInternalServerError(fmt.Sprintf("create assistant message: %v", err))
	}

	return &interfaces.OpenAPIChatStreamJob{
		ChatCtx:           prepared.chatCtx,
		Session:           prepared.session,
		AssistantMessage:  assistantMsg,
		UserMessageID:     userMsg.ID,
		RequestID:         requestID,
		ExternalSessionID: prepared.externalSessionID,
		Mode:              prepared.mode,
		KnowledgeBaseID:   prepared.kbID,
		TenantID:          prepared.tenantID,
		Query:             prepared.query,
	}, nil
}

func (s *openAPIService) RunStreamQA(
	ctx context.Context, job *interfaces.OpenAPIChatStreamJob, eventBus *event.EventBus,
) error {
	if job == nil || eventBus == nil {
		return errors.NewInternalServerError("invalid stream job")
	}

	prepared := &preparedOpenAPIChat{
		chatCtx:  job.ChatCtx,
		session:  job.Session,
		kbID:     job.KnowledgeBaseID,
		query:    job.Query,
		mode:     job.Mode,
		tenantID: job.TenantID,
	}

	qaReq, err := buildOpenAPIQARequest(ctx, prepared, job.UserMessageID, job.AssistantMessage.ID)
	if err != nil {
		return err
	}

	if job.Mode == types.OpenAPIChatModeWikiQA {
		return s.sessionService.AgentQA(ctx, qaReq, eventBus)
	}
	return s.sessionService.KnowledgeQA(ctx, qaReq, eventBus)
}
