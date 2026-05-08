// Package server implements an MCP (Model Context Protocol) server that
// exposes hermind's conversation + event surface to external MCP
// hosts (Claude Desktop, Cursor, Zed, Cline, etc.). The transport is
// newline-delimited JSON-RPC 2.0 over stdio — the same shape used by
// the acp/stdio server.
package server

import "encoding/json"

// Tool describes one MCP tool entry. The shape follows the MCP spec's
// `Tool` object so hosts can consume it verbatim.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// BuiltinTools returns the static catalog exposed by the MCP server.
// Keep the order stable — Claude Desktop pins tool indices per server
// boot for caching purposes.
func BuiltinTools() []Tool {
	obj := func(schema string) json.RawMessage { return json.RawMessage(schema) }
	return []Tool{
		{
			Name:        "conversations_list",
			Description: "List recent hermind conversations, optionally filtered by platform or free-text search.",
			InputSchema: obj(`{"type":"object","properties":{"platform":{"type":"string"},"limit":{"type":"integer","default":50},"search":{"type":"string"}}}`),
		},
		{
			Name:        "conversation_get",
			Description: "Fetch metadata and token stats for a single conversation by session key.",
			InputSchema: obj(`{"type":"object","required":["session_key"],"properties":{"session_key":{"type":"string"}}}`),
		},
		{
			Name:        "messages_read",
			Description: "Read the most recent messages from a conversation.",
			InputSchema: obj(`{"type":"object","required":["session_key"],"properties":{"session_key":{"type":"string"},"limit":{"type":"integer","default":50}}}`),
		},
		{
			Name:        "messages_send",
			Description: "Send a message to a target (platform:chat_id or session key).",
			InputSchema: obj(`{"type":"object","required":["target","message"],"properties":{"target":{"type":"string"},"message":{"type":"string"}}}`),
		},
		{
			Name:        "attachments_fetch",
			Description: "List attachments for a specific message.",
			InputSchema: obj(`{"type":"object","required":["session_key","message_id"],"properties":{"session_key":{"type":"string"},"message_id":{"type":"integer"}}}`),
		},
		{
			Name:        "events_poll",
			Description: "Non-blocking poll for new events since the given cursor.",
			InputSchema: obj(`{"type":"object","properties":{"after_cursor":{"type":"integer","default":0},"session_key":{"type":"string"},"limit":{"type":"integer","default":20}}}`),
		},
		{
			Name:        "events_wait",
			Description: "Block until a new event arrives or timeout_ms elapses.",
			InputSchema: obj(`{"type":"object","required":["after_cursor"],"properties":{"after_cursor":{"type":"integer"},"session_key":{"type":"string"},"timeout_ms":{"type":"integer","default":30000}}}`),
		},
		{
			Name:        "permissions_list_open",
			Description: "List pending permission requests that need a human decision.",
			InputSchema: obj(`{"type":"object"}`),
		},
		{
			Name:        "permissions_respond",
			Description: "Respond to a pending permission request.",
			InputSchema: obj(`{"type":"object","required":["id","decision"],"properties":{"id":{"type":"string"},"decision":{"type":"string","enum":["allow-once","allow-always","deny"]}}}`),
		},
		{
			Name:        "channels_list",
			Description: "List all known message targets grouped by platform.",
			InputSchema: obj(`{"type":"object","properties":{"platform":{"type":"string"}}}`),
		},
	}
}
