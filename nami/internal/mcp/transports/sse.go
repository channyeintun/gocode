package transports

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewSSE(definition Config) sdkmcp.Transport {
	return &sdkmcp.SSEClientTransport{
		Endpoint:   definition.URL,
		HTTPClient: newHeaderHTTPClient(definition.Headers),
	}
}
