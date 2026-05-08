package tool

import (
	"encoding/json"
	"fmt"
)

// ToolError encodes an error message as a JSON object: {"error": "msg"}.
// All tool handlers should return errors via this helper (or via (string, error)
// which Dispatch converts automatically) so the LLM sees structured output.
func ToolError(msg string) string {
	return mustJSON(map[string]any{"error": msg})
}

// ToolResult encodes data as a JSON string. If data is already a string,
// it is returned as-is (not double-encoded).
func ToolResult(data any) string {
	if s, ok := data.(string); ok {
		return s
	}
	return mustJSON(data)
}

// mustJSON marshals v and panics only if the input is unmarshalable
// (which means a programming error, not a user error).
func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":"tool: marshal: %s"}`, err.Error())
	}
	return string(data)
}

// newPanicError constructs an error from a recovered panic value.
func newPanicError(toolName string, p any) error {
	return fmt.Errorf("tool %q panicked: %v", toolName, p)
}
