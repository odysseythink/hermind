package mcp

// newTransport is the factory used by Hypervisor to spawn a transport
// for a configured server. All three Node-supported transports are
// implemented: stdio (PR-B), HTTP/streamable (PR-C), and SSE (PR-C).
func newTransport(srv *ServerConfig) (Transport, error) {
	switch parseServerType(srv) {
	case "stdio":
		return newStdioTransport(srv)
	case "http":
		return newHTTPTransport(srv)
	case "sse":
		return newSSETransport(srv)
	}
	return nil, ErrInvalidServerType
}
