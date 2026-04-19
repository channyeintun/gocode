package transports

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewHTTP(definition Config) sdkmcp.Transport {
	return &sdkmcp.StreamableClientTransport{
		Endpoint:   definition.URL,
		HTTPClient: newHeaderHTTPClient(definition.Headers),
	}
}
