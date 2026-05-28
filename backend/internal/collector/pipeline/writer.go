package pipeline

import (
	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

// Writer persists processed documents to the server storage directories.
type Writer struct {
	storageDir string
}

// NewWriter creates a new Writer.
func NewWriter(storageDir string) *Writer {
	return &Writer{storageDir: storageDir}
}

// Write serializes the document to JSON in the appropriate storage folder.
func (w *Writer) Write(doc *core.Document, filename string, parseOnly bool) (*core.Document, error) {
	return utils.WriteToServerDocuments(w.storageDir, doc, filename, parseOnly)
}
