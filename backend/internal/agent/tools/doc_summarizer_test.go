package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

type mockDocumentLister struct {
	docs []models.WorkspaceDocument
	err  error
}

func (m *mockDocumentLister) ListDocuments(ctx context.Context, folder string) ([]models.WorkspaceDocument, error) {
	return m.docs, m.err
}

type mockSummarizerLM struct {
	reply string
}

func (m *mockSummarizerLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return &core.Response{
		Message: core.Message{Content: core.NewTextContent(m.reply)},
	}, nil
}
func (m *mockSummarizerLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockSummarizerLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockSummarizerLM) Provider() string { return "mock" }
func (m *mockSummarizerLM) Model() string    { return "mock" }

func TestDocSummarizer_List_ReturnsDocuments(t *testing.T) {
	mock := &mockDocumentLister{
		docs: []models.WorkspaceDocument{
			{Filename: "report.pdf", DocId: "doc-1"},
			{Filename: "notes.txt", DocId: "doc-2"},
		},
	}
	tc := &ToolContext{
		Ctx:       context.Background(),
		DocSvc:    mock,
		LM:        &mockSummarizerLM{},
		Emit:      func(string) {},
		Workspace: &models.Workspace{ID: 1},
	}

	entry := NewDocSummarizerSkill(tc)
	args := json.RawMessage(`{"action":"list"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "report.pdf")
	require.Contains(t, result, "doc-1")
	require.Contains(t, result, "notes.txt")
}

func TestDocSummarizer_Summarize_ReturnsLLMReply(t *testing.T) {
	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(docPath, []byte("This is a long document about Go programming."), 0644))

	mock := &mockDocumentLister{
		docs: []models.WorkspaceDocument{
			{Filename: "test.txt", DocId: "doc-1", Docpath: docPath},
		},
	}
	lm := &mockSummarizerLM{reply: "- bullet one\n- bullet two"}
	tc := &ToolContext{
		Ctx:       context.Background(),
		DocSvc:    mock,
		LM:        lm,
		Emit:      func(string) {},
		Workspace: &models.Workspace{ID: 1},
	}

	entry := NewDocSummarizerSkill(tc)
	args := json.RawMessage(`{"action":"summarize","document_filename":"test.txt"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err)
	require.Contains(t, result, "bullet one")
	require.Contains(t, result, "bullet two")
}

func TestDocSummarizer_Summarize_NonexistentFile_ReturnsError(t *testing.T) {
	mock := &mockDocumentLister{docs: []models.WorkspaceDocument{}}
	tc := &ToolContext{
		Ctx:       context.Background(),
		DocSvc:    mock,
		LM:        &mockSummarizerLM{},
		Emit:      func(string) {},
		Workspace: &models.Workspace{ID: 1},
	}

	entry := NewDocSummarizerSkill(tc)
	args := json.RawMessage(`{"action":"summarize","document_filename":"missing.pdf"}`)
	result, err := entry.Handler(context.Background(), args)
	require.NoError(t, err) // tool.Error returns JSON, not Go error
	require.Contains(t, result, "error")
	require.Contains(t, result, "not found")
}

func TestDocSummarizer_DispatchViaRegistry(t *testing.T) {
	mock := &mockDocumentLister{
		docs: []models.WorkspaceDocument{
			{Filename: "report.pdf", DocId: "doc-1"},
		},
	}
	tc := &ToolContext{
		Ctx:       context.Background(),
		DocSvc:    mock,
		LM:        &mockSummarizerLM{},
		Emit:      func(string) {},
		Workspace: &models.Workspace{ID: 1},
	}

	entry := NewDocSummarizerSkill(tc)
	reg := tool.NewRegistry()
	reg.Register(entry)

	result, err := reg.Dispatch(context.Background(), "document-summarizer", json.RawMessage(`{"action":"list"}`))
	require.NoError(t, err)
	require.Contains(t, result, "report.pdf")
}
