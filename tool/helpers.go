package tool

import ptool "github.com/odysseythink/pantheon/tool"

// ToolError returns a JSON-encoded tool error. Re-exported for
// existing call sites that use the hermind name.
func ToolError(msg string) string { return ptool.Error(msg) }

// ToolResult returns a JSON-encoded tool success payload.
func ToolResult(data any) string { return ptool.Result(data) }
