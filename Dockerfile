# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.23.2-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    make \
    bash \
    git \
    ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Install oapi-codegen
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Copy the rest of the source code
COPY . .

# Generate SDK from OpenAPI specification
RUN chmod +x ./scripts/generate-sdk.sh && \
    ./scripts/generate-sdk.sh

# Build the binary with optimizations
# CGO_ENABLED=0 for static binary
# -ldflags for smaller binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev} -X main.commit=${COMMIT:-unknown} -X main.date=${BUILD_DATE:-unknown}" \
    -o linkwarden-mcp-server \
    ./cmd/linkwarden-mcp-server

# Final stage - minimal Alpine image
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -g 1000 mcpserver && \
    adduser -D -u 1000 -G mcpserver mcpserver

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/linkwarden-mcp-server /app/linkwarden-mcp-server

# Change ownership to non-root user
RUN chown -R mcpserver:mcpserver /app

# Switch to non-root user
USER mcpserver

# Set default environment variables (can be overridden)
ENV LINKWARDEN_BASE_URL="" \
    LINKWARDEN_TOKEN="" \
    TOOLSETS="" \
    READ_ONLY="false" \
    LOG_FILE=""

# Expose no ports (stdio-based communication)
# The MCP server communicates via stdin/stdout

# Set entrypoint
ENTRYPOINT ["/app/linkwarden-mcp-server", "stdio"]

# Health check is not applicable for stdio transport
# Labels for metadata
LABEL org.opencontainers.image.title="Linkwarden MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Linkwarden" \
      org.opencontainers.image.source="https://github.com/irfansofyana/linkwarden-mcp-server" \
      org.opencontainers.image.vendor="irfansofyana" \
      org.opencontainers.image.licenses="MIT"
