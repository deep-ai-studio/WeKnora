package handler

import (
	"context"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

func (h *OpenAPIHandler) chatStream(c *gin.Context, ctx context.Context, client *types.OpenAPIClient, req *types.OpenAPIChatRequest) {
	job, err := h.openAPIService.PrepareChatStream(ctx, client, req)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.Errorf(ctx, "[open-api] prepare stream failed: client=%s err=%v", client.ID, err)
		c.Error(apperrors.NewInternalServerError("prepare stream failed").WithDetails(err.Error()))
		return
	}

	session.SetSSEHeaders(c)

	receivedAt := time.Now()
	eventBus := event.NewEventBus()
	asyncCtx, cancel := context.WithCancel(logger.CloneContext(job.ChatCtx))
	defer cancel()

	streamHandler := session.NewAgentStreamHandler(
		asyncCtx,
		job.Session.ID,
		job.AssistantMessage.ID,
		job.RequestID,
		receivedAt,
		job.AssistantMessage,
		h.streamManager,
		eventBus,
	)
	streamHandler.Subscribe()

	session.AppendAgentQueryEvent(ctx, h.streamManager, job.Session.ID, job.AssistantMessage.ID)
	h.appendOpenAPIMetaEvent(ctx, job)

	go func() {
		if err := h.openAPIService.RunStreamQA(asyncCtx, job, eventBus); err != nil {
			logger.Errorf(asyncCtx, "[open-api] stream QA error: %v", err)
			eventBus.Emit(asyncCtx, event.Event{
				Type:      event.EventError,
				SessionID: job.Session.ID,
				Data: event.ErrorData{
					Error:     err.Error(),
					Stage:     "open_api_stream_qa",
					SessionID: job.Session.ID,
				},
			})
		}
	}()

	h.pollOpenAPIStream(c, ctx, job)
}

func (h *OpenAPIHandler) appendOpenAPIMetaEvent(ctx context.Context, job *interfaces.OpenAPIChatStreamJob) {
	meta := interfaces.StreamEvent{
		ID:        "open-api-meta-" + job.RequestID,
		Type:      types.ResponseTypeOpenAPIMeta,
		Done:      true,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"session_id":          job.Session.ID,
			"external_session_id": job.ExternalSessionID,
			"assistant_message_id": job.AssistantMessage.ID,
			"mode":                job.Mode,
			"knowledge_base_id":   job.KnowledgeBaseID,
		},
	}
	if err := h.streamManager.AppendEvent(ctx, job.Session.ID, job.AssistantMessage.ID, meta); err != nil {
		logger.Warnf(ctx, "[open-api] append meta event failed: %v", err)
	}
}

func (h *OpenAPIHandler) pollOpenAPIStream(c *gin.Context, ctx context.Context, job *interfaces.OpenAPIChatStreamJob) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastOffset := 0
	log := logger.GetLogger(ctx)

	for {
		select {
		case <-c.Request.Context().Done():
			log.Infof("[open-api] client disconnected session=%s message=%s", job.Session.ID, job.AssistantMessage.ID)
			return

		case <-ticker.C:
			events, newOffset, err := h.streamManager.GetEvents(ctx, job.Session.ID, job.AssistantMessage.ID, lastOffset)
			if err != nil {
				log.Warnf("[open-api] get stream events: %v", err)
				continue
			}

			streamCompleted := false
			for _, evt := range events {
				if evt.Type == types.ResponseTypeComplete || evt.Type == types.ResponseTypeError {
					streamCompleted = true
				}
				if c.Request.Context().Err() != nil {
					return
				}
				c.SSEvent("message", session.BuildStreamResponse(evt, job.RequestID))
				c.Writer.Flush()
			}
			lastOffset = newOffset

			if streamCompleted {
				log.Infof("[open-api] stream completed session=%s message=%s", job.Session.ID, job.AssistantMessage.ID)
				return
			}
		}
	}
}
