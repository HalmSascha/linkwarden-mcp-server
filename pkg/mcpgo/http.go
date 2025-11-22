package mcpgo

import (
    "net/http"

    "github.com/mark3labs/mcp-go/server"
)

type mark3labsHTTPImpl struct {
    mcpHTTPServer *server.StreamableHTTPServer
}

func NewHTTPServer(mcpServer Server) (*mark3labsHTTPImpl, error) {
    sImpl, ok := mcpServer.(*Mark3labsImpl)
    if !ok {
        return nil, ErrInvalidServerImplementation
    }
    return &mark3labsHTTPImpl{
        mcpHTTPServer: server.NewStreamableHTTPServer(sImpl.McpServer),
    }, nil
}

func (s *mark3labsHTTPImpl) Start(addr string) error {
    return s.mcpHTTPServer.Start(addr)
}

func (s *mark3labsHTTPImpl) Handler() http.Handler {
    return s.mcpHTTPServer
}