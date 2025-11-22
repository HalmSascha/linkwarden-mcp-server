## Summary
Enable all tools globally on the server, but filter `tools/list` and gate `tools/call` per client via a transport-level middleware, keeping multi-client support without per-session server instances.

## Approach
- Keep a single MCP server instance with all toolsets registered.
- Add an HTTP transport layer (or gateway) that:
  - Reads per-client allowed toolsets from headers/claims.
  - Filters `tools/list` response to only those allowed for the client.
  - Enforces `tools/call` by rejecting calls to disallowed tools.
- Avoid per-session server creation; use a shared server and a policy middleware.

## Policy Source (Client-Specified)
- Header: `X-Allowed-Toolsets: search,link,tags` (or `X-Allowed-Tools` for explicit tool names).
- Optional: `X-Read-Only: true` to disable write operations per client.
- Auth: `Authorization: Bearer <linkwarden-token>` and optional `X-Linkwarden-BaseURL` for multi-instance.

## Middleware Behavior
- `initialize`: Pass through.
- `tools/list`:
  - Use an internal registry of all tools (captured at registration time) to construct the full list.
  - Filter by client’s allowed toolsets and read-only flag before returning.
- `tools/call`:
  - Check requested tool against allowed set; reject if disallowed.
  - Forward allowed calls to the underlying MCP server dispatcher.
- Other methods: Pass through.

## Implementation Notes
- Tool registry: Capture name/description/params when `AddTools` is called; store in memory for listing.
- Hooks: Existing `mcp-go` hooks are used for logging; since they don’t modify results, filtering is done in the transport middleware.
- Read-only per client: When building the filtered list, omit write tools; also block write calls in `tools/call`.
- Superset enforcement: The allowed set is intersected with server-available tools; clients cannot request non-existent tools.

## Multi-Client Support
- Stateless per request: Each HTTP request carries its own allowed-toolset policy; no server-side global config.
- Optional session caching: Cache policy keyed by `Session-ID` header to avoid re-parsing for multi-call sessions.

## Security & Ops
- Never log raw tokens; log hashed identifiers for correlation.
- Enforce HTTPS and optional `X-Server-Token` if you want server access control.
- Rate limit per client to protect upstream Linkwarden.

## Stdio Mode
- For stdio-based clients, per-client limitation is already possible by passing env/flags when launching the server process; middleware filtering mainly targets the shared HTTP transport.

## Acceptance Criteria
- Two concurrent clients with different `X-Allowed-Toolsets` see different `tools/list` results.
- Disallowed tools return an error on `tools/call`.
- Read-only header removes write tools from `tools/list` and blocks writes.
- No per-session server instances are created.

## Testing
- Unit: middleware filtering logic and intersection with available tools.
- Integration: concurrent clients hitting `tools/list` and `tools/call` with distinct policies.
- Security: header parsing, token hashing in logs, rate-limit behavior.

## Next Steps
- Implement the HTTP transport and the JSON-RPC middleware.
- Add examples and docs showing required headers and client usage.
- Optional: support JWT claims for allowed toolsets instead of plaintext headers.