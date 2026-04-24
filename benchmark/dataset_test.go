package benchmark

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type genStubProvider struct{ reply string }

func (g *genStubProvider) Name() string { return "stub" }
func (g *genStubProvider) Complete(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	return &provider.Response{Message: message.Message{Role: message.RoleAssistant, Content: message.TextContent(g.reply)}}, nil
}
func (g *genStubProvider) Stream(_ context.Context, _ *provider.Request) (provider.Stream, error) {
	panic("not used")
}
func (g *genStubProvider) ModelInfo(_ string) *provider.ModelInfo {
	return nil
}
func (g *genStubProvider) EstimateTokens(_ string, _ string) (int, error) {
	return 0, nil
}
func (g *genStubProvider) Available() bool {
	return true
}

func TestGenerateWritesMetaAndRows(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "ds.jsonl")

	p := &genStubProvider{reply: `[{"id":"gen_1","category":"coding","message":"foo"},{"id":"gen_2","category":"reasoning","message":"bar"}]`}

	err := Generate(context.Background(), GenerateConfig{
		Count: 3, Seed: 42, OutPath: out, Provider: p, Model: "claude-stub",
	})
	require.NoError(t, err)

	f, err := os.Open(out)
	require.NoError(t, err)
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.Len(t, lines, 3) // 1 meta + 2 rows

	var meta struct {
		Meta DatasetMeta `json:"__meta"`
	}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &meta))
	assert.Equal(t, int64(42), meta.Meta.Seed)
	assert.Equal(t, 2, meta.Meta.Count)

	var item InputItem
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &item))
	assert.Equal(t, "gen_1", item.ID)
}
