package linkwardenmcp

import (
    "context"
    "strings"

    "github.com/irfansofyana/linkwarden-mcp-server/pkg/contextkey"
    "github.com/irfansofyana/linkwarden-mcp-server/pkg/mcpgo"
)

func EnforcePolicy(ctx context.Context, toolset string, isWrite bool) (*mcpgo.ToolResult, error) {
    allowed := contextkey.AllowedToolsetsFromContext(ctx)
    readOnly := contextkey.ReadOnlyFromContext(ctx)

    if len(allowed) > 0 {
        ok := false
        for _, ts := range allowed {
            if strings.EqualFold(strings.TrimSpace(ts), toolset) {
                ok = true
                break
            }
        }
        if !ok {
            return mcpgo.NewToolResultError("toolset not allowed"), nil
        }
    }

    if readOnly && isWrite {
        return mcpgo.NewToolResultError("operation not permitted in read-only mode"), nil
    }

    return nil, nil
}