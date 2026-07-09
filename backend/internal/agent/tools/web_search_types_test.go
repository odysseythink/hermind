package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCitation_JSONShape(t *testing.T) {
	c := Citation{
		ID:          "https://example.com",
		Title:       "Example",
		Text:        "A snippet",
		ChunkSource: "link://https://example.com",
	}
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "https://example.com", got["id"])
	require.Equal(t, "Example", got["title"])
	require.Equal(t, "A snippet", got["text"])
	require.Equal(t, "link://https://example.com", got["chunkSource"])
	_, hasScore := got["score"]
	require.False(t, hasScore, "score is omitempty so nil score should not appear")
}

func TestSearchError_Error(t *testing.T) {
	err := &SearchError{
		Provider: "TestProvider",
		Message:  "connection refused",
		Cause:    context.DeadlineExceeded,
	}
	msg := err.Error()
	if !strings.Contains(msg, "[TestProvider]") {
		t.Errorf("expected provider name in error, got: %s", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("expected message in error, got: %s", msg)
	}
	if !strings.Contains(msg, "deadline exceeded") {
		t.Errorf("expected cause in error, got: %s", msg)
	}
}

func TestSearchError_Unwrap(t *testing.T) {
	inner := errors.New("inner failure")
	err := &SearchError{Provider: "X", Message: "msg", Cause: inner}
	if !errors.Is(err, inner) {
		t.Error("expected errors.Is to unwrap to inner cause")
	}
	if err.Unwrap() != inner {
		t.Error("Unwrap() should return Cause")
	}
}
