package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserSkill_Scrape_DataURL(t *testing.T) {
	tc := &ToolContext{Emit: func(string) {}}
	skill := NewWebScrapingSkill(tc)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"url":"data:text/html,<html><body><h1>Hello Browser</h1></body></html>"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "Hello Browser")
}

func TestBrowserSkill_SSRF(t *testing.T) {
	tc := &ToolContext{Emit: func(string) {}}
	skill := NewWebScrapingSkill(tc)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"url":"http://localhost:8080/secret"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}

func TestBrowserSkill_InvalidScheme(t *testing.T) {
	tc := &ToolContext{Emit: func(string) {}}
	skill := NewWebScrapingSkill(tc)
	result, err := skill.Handler(context.Background(), json.RawMessage(`{"url":"ftp://example.com"}`))
	require.NoError(t, err)
	assert.Contains(t, result, "error")
}
