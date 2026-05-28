package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDisabledSkills_EmptyAndMalformed(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"[]", []string{}},
		{"null", nil},
		{"[\"foo\"]", []string{"foo"}},
		{"not-json", nil},
		{"[\"rag-memory\",\"web-scraping\"]", []string{"rag-memory", "web-scraping"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDisabledSkills(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsDisabled(t *testing.T) {
	require.True(t, isDisabled("rag-memory", []string{"rag-memory", "web-scraping"}))
	require.False(t, isDisabled("rechart", []string{"rag-memory", "web-scraping"}))
	require.False(t, isDisabled("rag-memory", nil))
}
