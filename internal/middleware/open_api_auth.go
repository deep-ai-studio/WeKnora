package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

const openAPIKeyHeader = "X-Open-API-Key"

// OpenAPIAuth validates partner credentials and injects tenant + client context.
func OpenAPIAuth(
	openAPIService interfaces.OpenAPIService,
	tenantService interfaces.TenantService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := strings.TrimSpace(c.GetHeader(openAPIKeyHeader))
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing X-Open-API-Key"})
			c.Abort()
			return
		}

		client, err := openAPIService.ValidateClientByAPIKey(c.Request.Context(), apiKey)
		if err != nil {
			logger.Warnf(c.Request.Context(), "[open-api] auth failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid Open API key"})
			c.Abort()
			return
		}

		tenant, err := tenantService.GetTenantByID(c.Request.Context(), client.TenantID)
		if err != nil || tenant == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid tenant for Open API client"})
			c.Abort()
			return
		}

		c.Set(types.TenantIDContextKey.String(), client.TenantID)
		c.Set(types.TenantInfoContextKey.String(), tenant)
		c.Set(types.OpenAPIClientContextKey.String(), client)

		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, client.TenantID)
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
		ctx = context.WithValue(ctx, types.OpenAPIClientContextKey, client)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
