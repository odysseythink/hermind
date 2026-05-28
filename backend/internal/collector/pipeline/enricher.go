package pipeline

import (
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// Enricher fills in metadata and computes statistics for a Document.
type Enricher struct {
	tokenizer *utils.Tokenizer
}

// NewEnricher creates a new Enricher.
func NewEnricher(tokenizer *utils.Tokenizer) *Enricher {
	return &Enricher{tokenizer: tokenizer}
}

// Enrich populates DocAuthor, Description, WordCount, TokenCountEstimate,
// and Published fields on the document.
func (e *Enricher) Enrich(doc *core.Document, content string, filePath string, metadata map[string]string) {
	doc.DocAuthor = metadata["docAuthor"]
	if doc.DocAuthor == "" {
		doc.DocAuthor = "Unknown"
	}
	doc.Description = metadata["description"]
	if doc.Description == "" {
		doc.Description = "Unknown"
	}
	doc.DocSource = metadata["docSource"]
	if doc.DocSource == "" {
		doc.DocSource = "a text file uploaded by the user."
	}
	doc.ChunkSource = metadata["chunkSource"]
	doc.Title = metadata["title"]
	if doc.Title == "" {
		doc.Title = doc.Name
	}

	doc.WordCount = len(strings.Fields(content))
	doc.TokenCountEstimate = e.tokenizer.Count(content)
	doc.Published = utils.CreatedDate(filePath)
}
