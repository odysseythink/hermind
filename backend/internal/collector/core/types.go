package core

// ProcessResponse is the standard response from document processing endpoints.
type ProcessResponse struct {
	Filename  string     `json:"filename"`
	Success   bool       `json:"success"`
	Reason    string     `json:"reason"`
	Documents []Document `json:"documents"`
}

// Document represents a single processed document.
type Document struct {
	Location           string `json:"location"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	Title              string `json:"title"`
	DocAuthor          string `json:"docAuthor"`
	Description        string `json:"description"`
	DocSource          string `json:"docSource"`
	ChunkSource        string `json:"chunkSource"`
	Published          string `json:"published"`
	WordCount          int    `json:"wordCount"`
	TokenCountEstimate int    `json:"token_count_estimate"`
	PageContent        string `json:"pageContent,omitempty"`
	IsDirectUpload     bool   `json:"isDirectUpload"`
}

// LinkContentResponse is the response from the get-link utility endpoint.
type LinkContentResponse struct {
	URL     string `json:"url"`
	Success bool   `json:"success"`
	Content string `json:"content"`
}

// ExtensionResponse is the response from collector extension endpoints.
type ExtensionResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Reason  string                 `json:"reason"`
}

// ParseOptions provides additional options for the parse endpoint.
type ParseOptions struct {
	AbsolutePath string `json:"absolutePath,omitempty"`
}
