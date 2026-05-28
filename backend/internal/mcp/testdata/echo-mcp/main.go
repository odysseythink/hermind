package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer("echo-mcp", "0.0.1")

	s.AddTool(mcp.NewTool("echo",
		mcp.WithDescription("Echo text back"),
		mcp.WithString("text", mcp.Required()),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text := req.GetString("text", "")
		return mcp.NewToolResultText(text), nil
	})

	s.AddTool(mcp.NewTool("add",
		mcp.WithDescription("Add two integers"),
		mcp.WithNumber("a", mcp.Required()),
		mcp.WithNumber("b", mcp.Required()),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := req.GetFloat("a", 0)
		b := req.GetFloat("b", 0)
		return mcp.NewToolResultText(fmt.Sprintf("sum=%s", strconv.FormatFloat(a+b, 'f', -1, 64))), nil
	})

	s.AddTool(mcp.NewTool("slow_echo",
		mcp.WithDescription("Echo after a delay"),
		mcp.WithString("text", mcp.Required()),
		mcp.WithNumber("delay_ms", mcp.Required()),
	), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		text := req.GetString("text", "")
		delay := req.GetFloat("delay_ms", 0)
		time.Sleep(time.Duration(delay) * time.Millisecond)
		return mcp.NewToolResultText(text), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "serve error: %v\n", err)
	}
}
