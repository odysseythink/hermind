package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTxtExtractor is a minimal extractor for .txt files used only in tests.
type testTxtExtractor struct{}

func (e *testTxtExtractor) Supports(ext string) bool {
	return ext == ".txt"
}

func (e *testTxtExtractor) Extract(ctx context.Context, input ExtractInput) (*ExtractOutput, error) {
	content, err := os.ReadFile(input.FilePath)
	if err != nil {
		return nil, err
	}
	return &ExtractOutput{Content: string(content)}, nil
}

// mockExtractor is a generic mock extractor for pipeline integration tests.
type mockExtractor struct {
	supportedExts []string
	content       string
	err           error
}

func (e *mockExtractor) Supports(ext string) bool {
	for _, s := range e.supportedExts {
		if ext == s {
			return true
		}
	}
	return false
}

func (e *mockExtractor) Extract(ctx context.Context, input ExtractInput) (*ExtractOutput, error) {
	if e.err != nil {
		return nil, e.err
	}
	return &ExtractOutput{Content: e.content}, nil
}

// testRegistry is a minimal in-memory registry for tests.
type testRegistry struct {
	byExt map[string]ContentExtractor
}

func newTestRegistry() *testRegistry {
	return &testRegistry{byExt: make(map[string]ContentExtractor)}
}

func (r *testRegistry) Register(ext string, extractor ContentExtractor) {
	r.byExt[ext] = extractor
}

func (r *testRegistry) Get(ext string) ContentExtractor {
	return r.byExt[ext]
}

func TestPipeline_ProcessFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	p := NewPipeline(tmpDir, tmpDir, NewEnricher(tok))
	p.SetRegistry(newTestRegistry())

	resp := p.ProcessFile(context.Background(), "missing.txt", ProcessFileOptions{}, nil)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Reason, "does not exist")
}

func TestPipeline_ProcessFile_ValidTxt(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	// Create a text file in the watch directory.
	filePath := filepath.Join(watchDir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world foo bar"), 0644))

	p := NewPipeline(tmpDir, watchDir, NewEnricher(tok))
	reg := newTestRegistry()
	txtExt := &testTxtExtractor{}
	reg.Register(".txt", txtExt)
	p.SetRegistry(reg)
	p.RegisterTextExtractor(txtExt)

	resp := p.ProcessFile(context.Background(), "hello.txt", ProcessFileOptions{}, nil)
	require.True(t, resp.Success, "expected success, got reason: %s", resp.Reason)
	require.Len(t, resp.Documents, 1)

	doc := resp.Documents[0]
	assert.Equal(t, "hello world foo bar", doc.PageContent)
	assert.Equal(t, 4, doc.WordCount)
	assert.True(t, doc.TokenCountEstimate > 0)
	assert.Contains(t, doc.Location, "custom-documents/")
	assert.False(t, doc.IsDirectUpload)

	// Original file should have been cleaned up.
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err))
}

func TestPipeline_ProcessFile_PDF(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	filePath := filepath.Join(watchDir, "doc.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("%PDF-1.4 fake"), 0644))

	p := NewPipeline(tmpDir, watchDir, NewEnricher(tok))
	reg := newTestRegistry()
	reg.Register(".pdf", &mockExtractor{supportedExts: []string{".pdf"}, content: "extracted pdf text"})
	p.SetRegistry(reg)

	resp := p.ProcessFile(context.Background(), "doc.pdf", ProcessFileOptions{}, nil)
	require.True(t, resp.Success, "expected success, got reason: %s", resp.Reason)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "extracted pdf text", resp.Documents[0].PageContent)
}

func TestPipeline_ProcessFile_Image(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	filePath := filepath.Join(watchDir, "img.png")
	require.NoError(t, os.WriteFile(filePath, []byte("fake png"), 0644))

	p := NewPipeline(tmpDir, watchDir, NewEnricher(tok))
	reg := newTestRegistry()
	reg.Register(".png", &mockExtractor{supportedExts: []string{".png", ".jpg", ".jpeg", ".webp"}, content: "ocr text"})
	p.SetRegistry(reg)

	resp := p.ProcessFile(context.Background(), "img.png", ProcessFileOptions{}, nil)
	require.True(t, resp.Success, "expected success, got reason: %s", resp.Reason)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "ocr text", resp.Documents[0].PageContent)
}

func TestPipeline_ProcessFile_Audio(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	filePath := filepath.Join(watchDir, "audio.mp3")
	require.NoError(t, os.WriteFile(filePath, []byte("fake mp3"), 0644))

	p := NewPipeline(tmpDir, watchDir, NewEnricher(tok))
	reg := newTestRegistry()
	reg.Register(".mp3", &mockExtractor{supportedExts: []string{".mp3", ".wav", ".mp4", ".mpeg", ".ogg", ".oga", ".opus", ".m4a", ".webm"}, content: "transcribed text"})
	p.SetRegistry(reg)

	resp := p.ProcessFile(context.Background(), "audio.mp3", ProcessFileOptions{}, nil)
	require.True(t, resp.Success, "expected success, got reason: %s", resp.Reason)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "transcribed text", resp.Documents[0].PageContent)
}

func TestPipeline_ProcessFile_ExtractorError(t *testing.T) {
	tmpDir := t.TempDir()
	watchDir := t.TempDir()
	tok, err := utils.NewTokenizer()
	require.NoError(t, err)

	filePath := filepath.Join(watchDir, "bad.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("%PDF-1.4 fake"), 0644))

	p := NewPipeline(tmpDir, watchDir, NewEnricher(tok))
	reg := newTestRegistry()
	reg.Register(".pdf", &mockExtractor{supportedExts: []string{".pdf"}, err: fmt.Errorf("extraction failed")})
	p.SetRegistry(reg)

	resp := p.ProcessFile(context.Background(), "bad.pdf", ProcessFileOptions{}, nil)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Reason, "extraction failed")
}
