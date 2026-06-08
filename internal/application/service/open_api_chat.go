package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
)

func (s *openAPIService) runQA(
	ctx context.Context,
	session *types.Session,
	kbID, query, mode string,
	tenantID uint64,
) (answer string, isFallback bool, references []types.OpenAPIReference, err error) {
	eventBus := event.NewEventBus()

	var (
		answerMu       sync.Mutex
		answerBuilder  strings.Builder
		qaErr          error
		isFallbackFlag bool
		referencesData interface{}
	)
	done := make(chan struct{})
	var closeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }

	eventBus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		answerBuilder.WriteString(data.Content)
		if data.IsFallback {
			isFallbackFlag = true
		}
		answerMu.Unlock()
		if data.Done {
			closeDone()
		}
		return nil
	})

	eventBus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentReferencesData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		referencesData = data.References
		answerMu.Unlock()
		return nil
	})

	eventBus.On(event.EventError, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.ErrorData)
		if !ok {
			return nil
		}
		logger.Errorf(ctx, "[open-api] QA error: %s", data.Error)
		answerMu.Lock()
		qaErr = fmt.Errorf("QA pipeline error: %s", data.Error)
		answerMu.Unlock()
		closeDone()
		return nil
	})

	requestID := uuid.New().String()
	now := time.Now()

	userMsg, err := s.messageService.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "user",
		Content:     query,
		RequestID:   requestID,
		CreatedAt:   now,
		IsCompleted: true,
		Channel:     types.ChannelAPI,
	})
	if err != nil {
		return "", false, nil, errors.NewInternalServerError(fmt.Sprintf("create user message: %v", err))
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, &types.Message{
		SessionID:   session.ID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   now,
		IsCompleted: false,
		Channel:     types.ChannelAPI,
	})
	if err != nil {
		return "", false, nil, errors.NewInternalServerError(fmt.Sprintf("create assistant message: %v", err))
	}

	qaReq := &types.QARequest{
		Session:            session,
		Query:              query,
		AssistantMessageID: assistantMsg.ID,
		UserMessageID:      userMsg.ID,
		KnowledgeBaseIDs:   []string{kbID},
	}

	if mode == types.OpenAPIChatModeWikiQA {
		agent := types.GetBuiltinAgentWithContext(ctx, types.BuiltinWikiResearcherID, tenantID)
		if agent == nil {
			return "", false, nil, errors.NewInternalServerError("wiki-qa agent not available")
		}
		qaReq.CustomAgent = agent
	}

	go func() {
		var execErr error
		if mode == types.OpenAPIChatModeWikiQA {
			execErr = s.sessionService.AgentQA(ctx, qaReq, eventBus)
		} else {
			execErr = s.sessionService.KnowledgeQA(ctx, qaReq, eventBus)
		}
		if execErr != nil {
			logger.Errorf(ctx, "[open-api] %s execution error: %v", mode, execErr)
			answerMu.Lock()
			qaErr = fmt.Errorf("QA execution error: %w", execErr)
			answerMu.Unlock()
			closeDone()
		}
	}()

	select {
	case <-done:
	case <-ctx.Done():
		assistantMsg.Content = "抱歉，回答已被取消。"
		assistantMsg.IsCompleted = true
		if updateErr := s.messageService.UpdateMessage(context.WithoutCancel(ctx), assistantMsg); updateErr != nil {
			logger.Warnf(ctx, "[open-api] failed to update cancelled assistant message: %v", updateErr)
		}
		return "", false, nil, errors.NewInternalServerError("request cancelled")
	}

	answerMu.Lock()
	answer = answerBuilder.String()
	qaError := qaErr
	isFallback = isFallbackFlag
	rawRefs := referencesData
	answerMu.Unlock()

	// wiki-qa (AgentQA) does not surface RAG chunk citations in the web UI; rag-qa slim-cites like docInfo.
	if mode == types.OpenAPIChatModeWikiQA {
		references = nil
	} else {
		references = slimOpenAPIReferences(rawRefs)
	}

	if answer == "" && qaError != nil {
		return "", isFallback, references, errors.NewInternalServerError(qaError.Error())
	}
	if answer == "" {
		answer = "抱歉，我暂时无法回答这个问题。"
	}

	assistantMsg.Content = answer
	assistantMsg.IsCompleted = true
	assistantMsg.IsFallback = isFallback
	if err := s.messageService.UpdateMessage(ctx, assistantMsg); err != nil {
		logger.Warnf(ctx, "[open-api] failed to update assistant message: %v", err)
	}

	return answer, isFallback, references, nil
}
