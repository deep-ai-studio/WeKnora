package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// OpenAPIHandler exposes partner Open API endpoints.
type OpenAPIHandler struct {
	openAPIService interfaces.OpenAPIService
	streamManager  interfaces.StreamManager
}

// NewOpenAPIHandler wires the Open API HTTP handler.
func NewOpenAPIHandler(openAPIService interfaces.OpenAPIService, streamManager interfaces.StreamManager) *OpenAPIHandler {
	return &OpenAPIHandler{
		openAPIService: openAPIService,
		streamManager:  streamManager,
	}
}

// Chat godoc
// @Summary      外部 Open API 问答
// @Description  使用 X-Open-API-Key 进行认证，按 external_user_id 隔离会话。默认 wiki-qa 模式（内置维基问答智能体）；可选 mode=rag-qa 走经典 RAG。stream=true 时返回 SSE（与前端 agent-chat 相同事件格式）。
// @Tags         Open API
// @Accept       json
// @Produce      json
// @Produce      text/event-stream
// @Param        X-Open-API-Key  header  string                 true  "Partner API key (sk-open-...)"
// @Param        request         body    types.OpenAPIChatRequest true  "问答请求"
// @Success      200  {object}  map[string]interface{}
// @Router       /open/chat [post]
func (h *OpenAPIHandler) Chat(c *gin.Context) {
	ctx := c.Request.Context()
	client, ok := types.OpenAPIClientFromContext(ctx)
	if !ok {
		c.Error(apperrors.NewUnauthorizedError("Open API client missing from context"))
		return
	}

	var req types.OpenAPIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}

	if req.Stream {
		h.chatStream(c, ctx, client, &req)
		return
	}

	resp, err := h.openAPIService.Chat(ctx, client, &req)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.Errorf(ctx, "[open-api] chat failed: client=%s err=%v", client.ID, err)
		c.Error(apperrors.NewInternalServerError("chat failed").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// CreateClient godoc
// @Summary      创建 Open API 客户端
// @Description  Admin+ 创建 partner 凭证，明文 key 仅返回一次
// @Tags         Open API
// @Accept       json
// @Produce      json
// @Param        request  body  types.CreateOpenAPIClientRequest  true  "创建请求"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /open-api/clients [post]
func (h *OpenAPIHandler) CreateClient(c *gin.Context) {
	ctx := c.Request.Context()

	var req types.CreateOpenAPIClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}

	resp, err := h.openAPIService.CreateClient(ctx, &req)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.Errorf(ctx, "[open-api] create client failed: err=%v", err)
		c.Error(apperrors.NewInternalServerError("failed to create Open API client").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    resp,
	})
}

// ListClients godoc
// @Summary      列出 Open API 客户端
// @Tags         Open API
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /open-api/clients [get]
func (h *OpenAPIHandler) ListClients(c *gin.Context) {
	ctx := c.Request.Context()

	clients, err := h.openAPIService.ListClients(ctx)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.Errorf(ctx, "[open-api] list clients failed: err=%v", err)
		c.Error(apperrors.NewInternalServerError("failed to list Open API clients").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    clients,
	})
}

// RevokeClient godoc
// @Summary      吊销 Open API 客户端
// @Tags         Open API
// @Produce      json
// @Param        id  path  string  true  "Client ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /open-api/clients/{id}/revoke [post]
func (h *OpenAPIHandler) RevokeClient(c *gin.Context) {
	ctx := c.Request.Context()
	clientID := c.Param("id")

	if err := h.openAPIService.RevokeClient(ctx, clientID); err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.Errorf(ctx, "[open-api] revoke client failed: id=%s err=%v", clientID, err)
		c.Error(apperrors.NewInternalServerError("failed to revoke Open API client").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
