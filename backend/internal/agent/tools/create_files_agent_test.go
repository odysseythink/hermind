package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func newCreateFilesTool(t *testing.T) (*tool.Entry, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		StorageDir:            t.TempDir(),
		AgentCreateFilesEnabled: true,
	}
	_ = os.MkdirAll(cfg.AgentCreateFilesDir, 0755)
	tc := &ToolContext{Cfg: cfg, Emit: func(string) {}}
	return NewCreateFilesAgentSkill(tc), cfg
}

func TestCreateFiles_AllFormats_BehaviorMatrix(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		content   any
		wantError bool
		errMatch  string
	}{
		{
			name:    "txt success",
			format:  "txt",
			content: "hello world",
		},
		{
			name:    "md success",
			format:  "md",
			content: "# Title\n\nBody",
		},
		{
			name:    "docx success",
			format:  "docx",
			content: "Document body",
		},
		{
			name:    "pdf success",
			format:  "pdf",
			content: "PDF body",
		},
		{
			name:    "xlsx success",
			format:  "xlsx",
			content: map[string]any{"sheets": []any{map[string]any{"name": "S1", "rows": []any{[]any{"A", "B"}}}}},
		},
		{
			name:      "pptx rejected",
			format:    "pptx",
			content:   "slides",
			wantError: true,
			errMatch:  "pptx format not supported",
		},
		{
			name:      "pdf non-ascii rejected",
			format:    "pdf",
			content:   "Hello 世界",
			wantError: true,
			errMatch:  "non-ASCII",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, _ := newCreateFilesTool(t)
			raw, _ := json.Marshal(map[string]any{
				"format":   tt.format,
				"filename": "test",
				"content":  tt.content,
			})
			result, err := entry.Handler(context.Background(), raw)
			require.NoError(t, err)

			if tt.wantError {
				require.Contains(t, result, tt.errMatch)
			} else {
				require.Contains(t, result, "saved_path")
				require.Contains(t, result, tt.format)
				var res map[string]any
				_ = json.Unmarshal([]byte(result), &res)
				path, _ := res["saved_path"].(string)
				require.FileExists(t, path)
			}
		})
	}
}

func TestCreateFiles_ExistingTxtMdTests_StillPass(t *testing.T) {
	entry, cfg := newCreateFilesTool(t)

	raw, _ := json.Marshal(map[string]any{
		"format":   "txt",
		"filename": "myfile",
		"content":  "hello",
	})
	result, err := entry.Handler(context.Background(), raw)
	require.NoError(t, err)
	require.Contains(t, result, "saved_path")

	var res map[string]any
	_ = json.Unmarshal([]byte(result), &res)
	path, _ := res["saved_path"].(string)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
	require.Contains(t, path, cfg.AgentCreateFilesDir)
}
