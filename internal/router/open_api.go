package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// RegisterOpenAPIRoutes registers partner-facing Open API routes.
// Must be called BEFORE the global Auth middleware; OpenAPIAuth handles
// X-Open-API-Key validation. The path is also listed in auth noAuthAPI
// so the global Auth middleware does not reject unauthenticated requests.
func RegisterOpenAPIRoutes(
	r *gin.Engine,
	openAPIHandler *handler.OpenAPIHandler,
	openAPIService interfaces.OpenAPIService,
	tenantService interfaces.TenantService,
) {
	open := r.Group("/api/v1/open")
	open.Use(middleware.OpenAPIAuth(openAPIService, tenantService))
	{
		open.POST("/chat", openAPIHandler.Chat)
	}
}

// RegisterOpenAPIAdminRoutes registers tenant-admin endpoints for managing
// partner credentials. Requires JWT auth and Admin+ RBAC.
func RegisterOpenAPIAdminRoutes(r *gin.RouterGroup, openAPIHandler *handler.OpenAPIHandler, g *rbacGuards) {
	clients := r.Group("/open-api/clients")
	{
		clients.POST("", g.Admin(), openAPIHandler.CreateClient)
		clients.GET("", g.Admin(), openAPIHandler.ListClients)
		clients.POST("/:id/revoke", g.Admin(), openAPIHandler.RevokeClient)
	}
}
