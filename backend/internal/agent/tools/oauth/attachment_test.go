package oauth

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/stretchr/testify/require"
)

func TestParseAttachments_NoAttachments_ReturnsEmpty(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	result, err := ParseAttachments(context.Background(), coll, nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestParseAttachments_HappyPath_AppendsParsedText(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	atts := []Attachment{
		{Filename: "hello.txt", DataBase64: base64.StdEncoding.EncodeToString([]byte("Hello world"))},
	}
	result, err := ParseAttachments(context.Background(), coll, atts)
	require.NoError(t, err)
	require.Contains(t, result, "--- Attached file: hello.txt ---")
	require.Contains(t, result, "Hello world")
}

func TestParseAttachments_InvalidBase64_ReturnsError(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	atts := []Attachment{
		{Filename: "bad.txt", DataBase64: "!!!not-base64!!!"},
	}
	_, err = ParseAttachments(context.Background(), coll, atts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode bad.txt")
}

func TestParseAttachments_ExceedsSizeCap_ReturnsError(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	atts := []Attachment{
		{Filename: "huge.txt", DataBase64: base64.StdEncoding.EncodeToString(make([]byte, MaxAttachmentBytes+1))},
	}
	_, err = ParseAttachments(context.Background(), coll, atts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}

func TestParseAttachments_MissingFilename_ReturnsError(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	atts := []Attachment{
		{Filename: "", DataBase64: base64.StdEncoding.EncodeToString([]byte("x"))},
	}
	_, err = ParseAttachments(context.Background(), coll, atts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing filename")
}

func TestParseAttachments_CollectorError_ReturnsError(t *testing.T) {
	// Use nil collector to force error
	atts := []Attachment{
		{Filename: "test.txt", DataBase64: base64.StdEncoding.EncodeToString([]byte("x"))},
	}
	_, err := ParseAttachments(context.Background(), nil, atts)
	require.Error(t, err)
}

func TestParseAttachments_MultipleFiles_Concatenated(t *testing.T) {
	coll, err := collector.NewLocalCollector(t.TempDir())
	require.NoError(t, err)
	defer coll.Close()

	atts := []Attachment{
		{Filename: "a.txt", DataBase64: base64.StdEncoding.EncodeToString([]byte("first"))},
		{Filename: "b.txt", DataBase64: base64.StdEncoding.EncodeToString([]byte("second"))},
	}
	result, err := ParseAttachments(context.Background(), coll, atts)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(result, "--- Attached file:"))
	require.Contains(t, result, "first")
	require.Contains(t, result, "second")
}
