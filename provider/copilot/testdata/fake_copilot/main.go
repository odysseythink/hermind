// Fake Copilot CLI used only in provider/copilot tests. It reads
// newline-delimited JSON-RPC frames from stdin and responds with
// pre-scripted frames based on the "method" field of each request.
//
// Supported methods:
//   - initialize            → returns {"result":{"protocolVersion":1}}
//   - session/new           → returns {"result":{"sessionId":"test-session"}}
//   - session/prompt        → returns an assistant message whose content
//                             includes a <tool_call>...</tool_call> block,
//                             then {"result":{"stopReason":"end_turn"}}
//
// If the first argument is "echo" the binary echoes every incoming line
// to stderr and terminates on EOF — useful for smoke testing the
// subprocess lifecycle without any protocol expectations.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "echo" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<16), 1<<22)
	out := bufio.NewWriter(os.Stdout)
	defer func() { _ = out.Flush() }()

	for in.Scan() {
		line := in.Bytes()
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(line, &req)
		switch req.Method {
		case "initialize":
			reply(out, req.ID, map[string]any{"protocolVersion": 1})
		case "session/new":
			reply(out, req.ID, map[string]any{"sessionId": "test-session"})
		case "session/prompt":
			reply(out, req.ID, map[string]any{"stopReason": "end_turn"})
		default:
			fmt.Fprintf(os.Stderr, "fake copilot: unknown method %s\n", req.Method)
		}
	}
}

func reply(w *bufio.Writer, id json.RawMessage, result any) {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	_, _ = w.Write(data)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}
