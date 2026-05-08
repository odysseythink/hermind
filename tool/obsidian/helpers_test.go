package obsidian

import (
	"context"
	"testing"
)

func TestParseFrontMatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTags int
		wantBody string
		wantErr  bool
	}{
		{
			name:     "with front-matter",
			input:    "---\ntags:\n  - ai\n  - note\n---\n\n# Hello\nBody",
			wantTags: 2,
			wantBody: "# Hello\nBody",
		},
		{
			name:     "without front-matter",
			input:    "# Hello\nBody",
			wantTags: 0,
			wantBody: "# Hello\nBody",
		},
		{
			name:     "malformed front-matter missing closing",
			input:    "---\ntags:\n  - ai\n# Hello",
			wantTags: 0,
			wantBody: "---\ntags:\n  - ai\n# Hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := parseFrontMatter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
			tags, _ := fm["tags"].([]any)
			if len(tags) != tt.wantTags {
				t.Errorf("tags count = %d, want %d", len(tags), tt.wantTags)
			}
		})
	}
}

func TestSerializeNoteRoundTrip(t *testing.T) {
	original := "---\ntags:\n  - ai\n---\n\n# Hello\nBody"
	fm, body, err := parseFrontMatter(original)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := serializeNote(fm, body)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	_, body2, err := parseFrontMatter(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if body2 != body {
		t.Errorf("body mismatch after round-trip: %q vs %q", body2, body)
	}
}

func TestResolveVaultPath(t *testing.T) {
	tests := []struct {
		name    string
		vault   string
		path    string
		wantErr bool
	}{
		{"valid subpath", "/home/user/vault", "Projects/Idea.md", false},
		{"escape via ..", "/home/user/vault", "../etc/passwd", true},
		{"escape nested", "/home/user/vault", "foo/../../etc/passwd", true},
		{"root vault valid", "/", "Notes/Idea.md", false},
		{"root vault escape", "/", "../etc/passwd", true},
		{"empty note path", "/home/user/vault", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveVaultPath(tt.vault, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveVaultPath(%q, %q) error = %v, wantErr %v", tt.vault, tt.path, err, tt.wantErr)
			}
			if !tt.wantErr && got == "" {
				t.Errorf("expected non-empty path, got empty")
			}
		})
	}
}

func TestExtractWikilinks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"basic", "See [[Note A]] and [[Note B]]", []string{"Note A", "Note B"}},
		{"with alias", "[[Note A|Alias]]", []string{"Note A"}},
		{"no links", "Plain text", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractWikilinks(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d links, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("link[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestVaultPathFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), VaultPathKey, "/my/vault")
	got, ok := vaultPathFromContext(ctx)
	if !ok || got != "/my/vault" {
		t.Errorf("vaultPathFromContext = %q, %v; want /my/vault, true", got, ok)
	}

	emptyCtx := context.Background()
	got, ok = vaultPathFromContext(emptyCtx)
	if ok || got != "" {
		t.Errorf("vaultPathFromContext(empty) = %q, %v; want empty, false", got, ok)
	}
}
