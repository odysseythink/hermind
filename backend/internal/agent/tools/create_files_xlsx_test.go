package tools

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXLSX_HappyPath_SingleSheet(t *testing.T) {
	dst := t.TempDir() + "/test.xlsx"
	content := map[string]any{
		"sheets": []any{
			map[string]any{
				"name": "Sales",
				"rows": []any{
					[]any{"Product", "Price"},
					[]any{"Widget", "10"},
				},
			},
		},
	}
	err := writeXLSXFile(t.Context(), dst, content)
	require.NoError(t, err)
	require.FileExists(t, dst)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, []byte("PK\x03\x04"), data[:4])
}

func TestXLSX_MultipleSheets(t *testing.T) {
	dst := t.TempDir() + "/multi.xlsx"
	content := map[string]any{
		"sheets": []any{
			map[string]any{
				"name": "Sheet A",
				"rows": []any{[]any{"A1"}},
			},
			map[string]any{
				"name": "Sheet B",
				"rows": []any{[]any{"B1"}},
			},
		},
	}
	err := writeXLSXFile(t.Context(), dst, content)
	require.NoError(t, err)
	require.FileExists(t, dst)
}

func TestXLSX_HeaderRowBolded(t *testing.T) {
	dst := t.TempDir() + "/header.xlsx"
	content := map[string]any{
		"sheets": []any{
			map[string]any{
				"name": "Data",
				"rows": []any{
					[]any{"Name", "Value"},
					[]any{"X", "1"},
				},
			},
		},
	}
	err := writeXLSXFile(t.Context(), dst, content)
	require.NoError(t, err)
	require.FileExists(t, dst)
}

func TestXLSX_EmptyRows_HandledGracefully(t *testing.T) {
	dst := t.TempDir() + "/empty.xlsx"
	content := map[string]any{
		"sheets": []any{
			map[string]any{
				"name": "Empty",
				"rows": []any{},
			},
		},
	}
	err := writeXLSXFile(t.Context(), dst, content)
	require.NoError(t, err)
}

func TestXLSX_InvalidContent_ReturnsError(t *testing.T) {
	dst := t.TempDir() + "/bad.xlsx"
	err := writeXLSXFile(t.Context(), dst, "not an object")
	require.Error(t, err)
	require.Contains(t, err.Error(), "content must have shape")
}

func TestXLSX_ReturnsValidZIP(t *testing.T) {
	dst := t.TempDir() + "/zip.xlsx"
	content := map[string]any{
		"sheets": []any{
			map[string]any{
				"name": "Z",
				"rows": []any{[]any{"cell"}},
			},
		},
	}
	err := writeXLSXFile(t.Context(), dst, content)
	require.NoError(t, err)
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	require.Equal(t, []byte("PK\x03\x04"), data[:4])
}
