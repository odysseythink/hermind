package skills

import pskills "github.com/odysseythink/pantheon/extensions/skills"

// Retriever is a re-export of pantheon's skill retriever.
type Retriever = pskills.Retriever

// NewRetriever constructs a Retriever.
func NewRetriever(skillDir string, embedder pskills.Embedder) *Retriever {
	return pskills.NewRetriever(skillDir, embedder)
}
