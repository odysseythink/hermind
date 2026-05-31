package embedder

import "github.com/gin-gonic/gin"

// NativeModelInfo describes a supported native embedding model.
type NativeModelInfo struct {
	ID                      string
	Name                    string
	Description             string
	Lang                    string
	Size                    string
	ModelCard               string
	HFRepo                  string
	Dimensions              int
	MaxConcurrentChunks     int
	EmbeddingMaxChunkLength int
	ChunkPrefix             string
	QueryPrefix             string
}

var nativeModels = map[string]NativeModelInfo{
	"sentence-transformers/all-MiniLM-L6-v2": {
		ID:                      "sentence-transformers/all-MiniLM-L6-v2",
		Name:                    "all-MiniLM-L6-v2",
		Description:             "A lightweight and fast model for embedding text. The default model for Hermind.",
		Lang:                    "English",
		Size:                    "23MB",
		ModelCard:               "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2",
		HFRepo:                  "sentence-transformers/all-MiniLM-L6-v2",
		Dimensions:              384,
		MaxConcurrentChunks:     25,
		EmbeddingMaxChunkLength: 1000,
		ChunkPrefix:             "",
		QueryPrefix:             "",
	},
	"sentence-transformers/all-MiniLM-L12-v2": {
		ID:                      "sentence-transformers/all-MiniLM-L12-v2",
		Name:                    "all-MiniLM-L12-v2",
		Description:             "A higher-quality lightweight model for embedding text with more layers.",
		Lang:                    "English",
		Size:                    "34MB",
		ModelCard:               "https://huggingface.co/sentence-transformers/all-MiniLM-L12-v2",
		HFRepo:                  "sentence-transformers/all-MiniLM-L12-v2",
		Dimensions:              384,
		MaxConcurrentChunks:     25,
		EmbeddingMaxChunkLength: 1000,
		ChunkPrefix:             "",
		QueryPrefix:             "",
	},
	"sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2": {
		ID:                      "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
		Name:                    "paraphrase-multilingual-MiniLM-L12-v2",
		Description:             "A multilingual embedding model supporting 50+ languages.",
		Lang:                    "50+ languages",
		Size:                    "118MB",
		ModelCard:               "https://huggingface.co/sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
		HFRepo:                  "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
		Dimensions:              384,
		MaxConcurrentChunks:     25,
		EmbeddingMaxChunkLength: 1000,
		ChunkPrefix:             "",
		QueryPrefix:             "",
	},
}

// AvailableModels returns the list of supported native embedding models
// in the format expected by the frontend.
func AvailableModels() []gin.H {
	models := make([]gin.H, 0, len(nativeModels))
	for _, info := range nativeModels {
		models = append(models, gin.H{
			"id":          info.ID,
			"name":        info.Name,
			"description": info.Description,
			"lang":        info.Lang,
			"size":        info.Size,
			"modelCard":   info.ModelCard,
		})
	}
	return models
}

func getNativeModelInfo(modelID string) (NativeModelInfo, bool) {
	if info, ok := nativeModels[modelID]; ok {
		return info, true
	}
	// Fallback to default
	info, ok := nativeModels["sentence-transformers/all-MiniLM-L6-v2"]
	return info, ok
}
