package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
)

func newFSToolContext(t *testing.T, root string) *ToolContext {
	return &ToolContext{
		Cfg: &config.Config{
			AgentFilesystemEnabled: true,
			AgentFilesystemRoot:    root,
		},
		Emit: func(string) {},
	}
}

// --- read-only tests ---

func TestFilesystem_ListDir_ReturnsFilesAndDirs(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0644)
	_ = os.Mkdir(filepath.Join(root, "subdir"), 0755)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"list_dir","path":""}`))
	require.NoError(t, err)
	require.Contains(t, result, "a.txt")
	require.Contains(t, result, "subdir")
}

func TestFilesystem_ListDir_PathNotFound_ReturnsToolError(t *testing.T) {
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"list_dir","path":"missing"}`))
	require.NoError(t, err)
	require.Contains(t, result, "error")
}

func TestFilesystem_ReadFile_HappyPath(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"read_file","path":"hello.txt"}`))
	require.NoError(t, err)
	require.Contains(t, result, "world")
}

func TestFilesystem_ReadFile_DirectoryReturnsToolError(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "subdir"), 0755)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"read_file","path":"subdir"}`))
	require.NoError(t, err)
	require.Contains(t, result, "directory")
}

func TestFilesystem_ReadFile_SizeCap_Truncates(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, fsMaxReadBytes+10)
	for i := range big {
		big[i] = 'x'
	}
	_ = os.WriteFile(filepath.Join(root, "big.txt"), big, 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"read_file","path":"big.txt"}`))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &payload))
	require.Equal(t, true, payload["truncated"])
}

func TestFilesystem_GetInfo_ReturnsStat(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "f.txt"), []byte("abc"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"get_info","path":"f.txt"}`))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &payload))
	require.Equal(t, float64(3), payload["size"])
	require.Equal(t, false, payload["is_dir"])
}

func TestFilesystem_SearchFiles_MatchesGlob(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("1"), 0644)
	_ = os.WriteFile(filepath.Join(root, "b.md"), []byte("2"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"search_files","path":"","pattern":"*.txt"}`))
	require.NoError(t, err)
	require.Contains(t, result, "a.txt")
	require.NotContains(t, result, "b.md")
}

// --- write tests ---

func TestFilesystem_WriteFile_HappyPath(t *testing.T) {
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"write_file","path":"hello.txt","content":"hi"}`))
	require.NoError(t, err)
	require.Contains(t, result, "bytes")
	b, _ := os.ReadFile(filepath.Join(root, "hello.txt"))
	require.Equal(t, "hi", string(b))
}

func TestFilesystem_WriteFile_CreatesMissingParents(t *testing.T) {
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	_, _ = e.Handler(context.Background(), json.RawMessage(`{"action":"write_file","path":"a/b/c.txt","content":"deep"}`))
	b, _ := os.ReadFile(filepath.Join(root, "a", "b", "c.txt"))
	require.Equal(t, "deep", string(b))
}

func TestFilesystem_EditFile_ReplacesOldString(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello world"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"edit_file","path":"f.txt","old_string":"world","new_string":"universe"}`))
	require.NoError(t, err)
	require.Contains(t, result, "replacements")
	b, _ := os.ReadFile(filepath.Join(root, "f.txt"))
	require.Equal(t, "hello universe", string(b))
}

func TestFilesystem_EditFile_OldStringNotFound_ReturnsToolError(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"edit_file","path":"f.txt","old_string":"missing","new_string":"x"}`))
	require.NoError(t, err)
	require.Contains(t, result, "not found")
}

func TestFilesystem_EditFile_AmbiguousMatch_ReturnsToolError(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "f.txt"), []byte("aa aa"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"edit_file","path":"f.txt","old_string":"aa","new_string":"b"}`))
	require.NoError(t, err)
	require.Contains(t, result, "ambiguous")
}

func TestFilesystem_MoveFile_HappyPath(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("data"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"move_file","source":"a.txt","destination":"b.txt"}`))
	require.NoError(t, err)
	require.Contains(t, result, "b.txt")
	require.NoFileExists(t, filepath.Join(root, "a.txt"))
	require.FileExists(t, filepath.Join(root, "b.txt"))
}

func TestFilesystem_CopyFile_HappyPath(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), []byte("data"), 0644)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"copy_file","source":"a.txt","destination":"b.txt"}`))
	require.NoError(t, err)
	require.Contains(t, result, "b.txt")
	require.FileExists(t, filepath.Join(root, "a.txt"))
	require.FileExists(t, filepath.Join(root, "b.txt"))
}

func TestFilesystem_CreateDir_HappyPath(t *testing.T) {
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"create_dir","path":"newdir"}`))
	require.NoError(t, err)
	require.Contains(t, result, "newdir")
	require.DirExists(t, filepath.Join(root, "newdir"))
}

func TestFilesystem_CreateDir_AlreadyExists_Idempotent(t *testing.T) {
	root := t.TempDir()
	_ = os.Mkdir(filepath.Join(root, "existing"), 0755)

	tc := newFSToolContext(t, root)
	e := NewFilesystemAgentSkill(tc)
	_, err := e.Handler(context.Background(), json.RawMessage(`{"action":"create_dir","path":"existing"}`))
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(root, "existing"))
}

func TestFilesystem_Write_TriggersApprovalGate(t *testing.T) {
	var calledWith string
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	tc.Approval = func(_ context.Context, name string, _ any, _ string) (bool, string) {
		calledWith = name
		return true, ""
	}
	e := NewFilesystemAgentSkill(tc)
	_, _ = e.Handler(context.Background(), json.RawMessage(`{"action":"write_file","path":"hello.txt","content":"hi"}`))
	require.Equal(t, "filesystem-agent:write_file", calledWith)
}

func TestFilesystem_Write_ApprovalRejected_NoFileWritten(t *testing.T) {
	root := t.TempDir()
	tc := newFSToolContext(t, root)
	tc.Approval = func(context.Context, string, any, string) (bool, string) {
		return false, "user denied"
	}
	e := NewFilesystemAgentSkill(tc)
	result, err := e.Handler(context.Background(), json.RawMessage(`{"action":"write_file","path":"hello.txt","content":"hi"}`))
	require.NoError(t, err)
	require.Contains(t, result, "rejected")
	require.NoFileExists(t, filepath.Join(root, "hello.txt"))
}

func TestFilesystem_CheckFn_FalseWhenDisabled(t *testing.T) {
	tc := &ToolContext{
		Cfg: &config.Config{AgentFilesystemEnabled: false},
	}
	e := NewFilesystemAgentSkill(tc)
	require.False(t, e.CheckFn())
}

func TestFilesystem_DispatchViaRegistry(t *testing.T) {
	root := t.TempDir()
	reg := tool.NewRegistry()
	reg.Register(NewFilesystemAgentSkill(newFSToolContext(t, root)))
	result, err := reg.Dispatch(context.Background(), "filesystem-agent", []byte(`{"action":"list_dir","path":""}`))
	require.NoError(t, err)
	require.NotEmpty(t, result)
}
