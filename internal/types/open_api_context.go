package types

import (
	"context"
)

// OpenAPIClientFromContext extracts the partner client set by Open API auth middleware.
func OpenAPIClientFromContext(ctx context.Context) (*OpenAPIClient, bool) {
	v, ok := ctx.Value(OpenAPIClientContextKey).(*OpenAPIClient)
	return v, ok && v != nil
}
