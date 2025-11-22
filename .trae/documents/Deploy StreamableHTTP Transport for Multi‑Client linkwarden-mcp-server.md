## Summary
- Implement MCP-Go StreamableHTTP transport for multi-client access.
- Authenticate exclusively via headers: `X-Linkwarden-Token` and `X-Linkwarden-BaseURL`.
- Optional headers for policy: `X-Toolsets`, `X-Read-Only`.
- Enforce toolset/read-only per request; no JWT, no server-side secrets.

## Current State
- Stdio-only server entrypoint (cmd/linkwarden-mcp-server/main.go:57–104, 106–154, 156–185).
- Server creation and tool registration (pkg/linkwardenmcp/server.go:13–46) with context client support (pkg/linkwardenmcp/server.go:48–69).
- Tools use the client from context (e.g., pkg/linkwardenmcp/link.go:63–71, 104–114).

## Header Scheme
- Required:
  - `X-Linkwarden-Token`: forwarded as `Authorization: Bearer <token>` to Linkwarden.
  - `X-Linkwarden-BaseURL`: Linkwarden instance URL.
- Optional:
  - `X-Toolsets`: comma-separated requested toolsets; server enforces known set only.
  - `X-Read-Only`: `true|false`; server enforces write blocking when true.

## Request Flow
1. StreamableHTTP `/mcp` receives request; transport passes HTTP headers to handlers.
2. Parse headers; validate required ones.
3. Build per-request Linkwarden client using `baseURL` and `token`; attach to context via `contextkey.WithClient`.
4. Enforce toolset and read-only via handler middleware; filter `listTools` via hooks.
5. Execute tool; return result. No data persisted.

## Implementation Steps
1. Add HTTP transport wrapper
- Create `pkg/mcpgo/http.go`: `NewHTTPServer(mcpServer)` wraps `server.NewStreamableHTTPServer` and `Start(addr)`. Expose `Handler()` to mount under `/mcp`.

2. Introduce `http` cobra command
- In `cmd/linkwarden-mcp-server/main.go`, add `httpCmd`:
  - Initialize logging/observability.
  - Build MCP server (`NewLinkwardenMcpServer`).
  - Start HTTP transport at `--addr` (default `:8080`).
  - Optionally mount health endpoints when using a custom mux.

3. Header parsing and context injection
- Add middleware to read `X-Linkwarden-Token`, `X-Linkwarden-BaseURL`.
- Construct `linkwarden.ClientWithResponses` via `WithRequestEditorFn` to set Authorization header.
- Inject client into `ctx` with `contextkey.WithClient` so existing tools resolve it.

4. Policy enforcement
- Tool handler middleware: block calls not in requested toolsets; enforce read-only (deny write tools).
- Hooks: filter `listTools` results to only permitted tools.
- Known toolset names come from the existing toolset grouping.

5. Configuration
- New flags/env for `http` command:
  - `--addr` (`MCP_HTTP_ADDR`), default `:8080`.
  - `--allow-unauth` for local dev (if true, allow missing headers with fallback to startup config).
  - `--rate-limit-rps` and `--rate-limit-burst` (optional in-memory limits per derived `client_id`).
- Keep `stdio` command unchanged.

6. Security
- Require TLS in production; do not log sensitive headers.
- Validate `X-Linkwarden-BaseURL` format; normalize toolset names.
- Constant-time comparisons where applicable.

7. Testing
- Unit tests for header parsing, error paths (missing/invalid headers).
- Integration tests: call `get_all_links` with different tokens/base URLs and verify isolation.
- Verify read-only blocks write tools; `listTools` shows only permitted tools.

## Deployment
- Run `http` command behind a reverse proxy terminating TLS.
- Horizontal scaling supported; stateless design.

## Rationale
- Simpler developer experience: users supply their Linkwarden token and base URL per request.
- No JWT complexity; optional policy via headers with server-side enforcement.
- Leverages MCP-Go StreamableHTTP’s header forwarding for clean per-request client construction.