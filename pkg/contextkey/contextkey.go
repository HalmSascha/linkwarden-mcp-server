package contextkey

import (
    "context"
)

// contextKey is a type used for context value keys to avoid key collisions.
type contextKey string

// Context keys for storing various values.
const (
    clientKey contextKey = "client"
    allowedToolsetsKey contextKey = "allowed_toolsets"
    readOnlyKey        contextKey = "read_only"
)

// WithClient returns a new context with the client instance attached.
func WithClient(ctx context.Context, client interface{}) context.Context {
    return context.WithValue(ctx, clientKey, client)
}

// ClientFromContext extracts the client instance from the context.
// Returns nil if no client is found.
func ClientFromContext(ctx context.Context) interface{} {
    return ctx.Value(clientKey)
}

func WithAllowedToolsets(ctx context.Context, toolsets []string) context.Context {
    return context.WithValue(ctx, allowedToolsetsKey, toolsets)
}

func AllowedToolsetsFromContext(ctx context.Context) []string {
    v := ctx.Value(allowedToolsetsKey)
    if v == nil {
        return nil
    }
    if ts, ok := v.([]string); ok {
        return ts
    }
    return nil
}

func WithReadOnly(ctx context.Context, readOnly bool) context.Context {
    return context.WithValue(ctx, readOnlyKey, readOnly)
}

func ReadOnlyFromContext(ctx context.Context) bool {
    v := ctx.Value(readOnlyKey)
    if v == nil {
        return false
    }
    if ro, ok := v.(bool); ok {
        return ro
    }
    return false
}
