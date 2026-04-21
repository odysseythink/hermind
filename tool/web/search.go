// tool/web/search.go
package web

// webSearchSchema is the JSON Schema for web_search tool arguments.
// The actual handler lives in search_dispatcher.go — this file holds
// only the shared schema string so the dispatcher and any future
// callers share a single source of truth.
const webSearchSchema = `{
  "type": "object",
  "properties": {
    "query":       { "type": "string", "description": "Search query" },
    "num_results": { "type": "number", "description": "Number of results to return (default 5, max 20)" }
  },
  "required": ["query"]
}`
