package chunker

import (
	"strings"
	"unicode"
)

// Chunker splits text into chunks with configurable size, overlap, and prefix.
type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
	ChunkPrefix  string
}

// NewChunker creates a Chunker with the given parameters.
func NewChunker(chunkSize, overlap int, prefix string) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: overlap,
		ChunkPrefix:  prefix,
	}
}

// Split splits text into chunks. It tries to preserve paragraph and sentence boundaries.
func (c *Chunker) Split(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if len(text) <= c.ChunkSize {
		return c.applyPrefix([]string{text})
	}

	paragraphs := splitParagraphs(text)
	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, p := range paragraphs {
		if p == "" {
			continue
		}
		// If paragraph alone exceeds chunk size, split it further
		if len(p) > c.ChunkSize {
			if current.Len() > 0 {
				flush()
			}
			chunks = append(chunks, c.splitLargeText(p)...)
			continue
		}
		// If adding this paragraph would exceed chunk size, flush current
		if current.Len() > 0 && current.Len()+1+len(p) > c.ChunkSize {
			flush()
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(p)
	}
	flush()

	chunks = c.applyOverlap(chunks)
	return c.applyPrefix(chunks)
}

// applyOverlap adds overlapping text between consecutive chunks.
// Each resulting chunk is capped at ChunkSize.
func (c *Chunker) applyOverlap(chunks []string) []string {
	if c.ChunkOverlap <= 0 || len(chunks) <= 1 {
		return chunks
	}
	out := make([]string, len(chunks))
	out[0] = chunks[0]
	for i := 1; i < len(chunks); i++ {
		prev := out[i-1]
		overlap := ""
		if len(prev) > c.ChunkOverlap {
			start := len(prev) - c.ChunkOverlap
			// Scan forward to find a clean word boundary
			for j := start; j < len(prev); j++ {
				if prev[j] == ' ' || prev[j] == '\n' {
					start = j + 1
				}
			}
			overlap = prev[start:]
		}
		combined := chunks[i]
		if overlap != "" {
			combined = overlap + "\n" + chunks[i]
		}
		// Ensure we don't exceed ChunkSize after adding overlap
		if len(combined) > c.ChunkSize {
			combined = combined[:c.ChunkSize]
			// Trim to last word boundary
			if idx := strings.LastIndexAny(combined, " \n"); idx > 0 {
				combined = combined[:idx]
			}
		}
		out[i] = strings.TrimSpace(combined)
	}
	return out
}

// splitLargeText splits a large text (single paragraph) into chunks.
func (c *Chunker) splitLargeText(text string) []string {
	// Try sentence boundaries first
	sentences := splitSentences(text)
	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, s := range sentences {
		if s == "" {
			continue
		}
		if len(s) > c.ChunkSize {
			if current.Len() > 0 {
				flush()
			}
			chunks = append(chunks, c.splitByWords(s)...)
			continue
		}
		if current.Len() > 0 && current.Len()+1+len(s) > c.ChunkSize {
			flush()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(s)
	}
	flush()

	return chunks
}

// splitByWords splits text into fixed-size chunks with word boundary preference.
func (c *Chunker) splitByWords(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, w := range words {
		if current.Len() > 0 && current.Len()+1+len(w) > c.ChunkSize {
			flush()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(w)
	}
	flush()

	return chunks
}

func (c *Chunker) applyPrefix(chunks []string) []string {
	if c.ChunkPrefix == "" {
		return chunks
	}
	for i := range chunks {
		chunks[i] = c.ChunkPrefix + chunks[i]
	}
	return chunks
}

// splitParagraphs splits text on blank lines.
func splitParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	var paragraphs []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			paragraphs = append(paragraphs, s)
		}
		current.Reset()
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
		} else {
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(line)
		}
	}
	flush()
	return paragraphs
}

// splitSentences splits text on sentence terminators (.!? followed by space or newline).
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimSpace(current.String())
		if s != "" {
			sentences = append(sentences, s)
		}
		current.Reset()
	}

	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			// Check if next non-space is uppercase or end of text
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == ' ' || next == '\n' || next == '\t' {
					// Look ahead for uppercase or quote
					for j := i + 2; j < len(runes); j++ {
						if unicode.IsSpace(runes[j]) {
							continue
						}
						if unicode.IsUpper(runes[j]) || runes[j] == '"' || runes[j] == '\'' {
							flush()
						}
						break
					}
				}
			} else {
				flush()
			}
		}
	}
	flush()
	return sentences
}
