package correlation

import (
	"context"
	"strings"
)

type ctxKey struct{}

var key ctxKey

func IntoContext(ctx context.Context, correlationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	correlationID = strings.TrimSpace(correlationID)
	if correlationID == "" {
		return ctx
	}
	return context.WithValue(ctx, key, correlationID)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if correlationID, ok := ctx.Value(key).(string); ok {
		return correlationID
	}
	return ""
}
