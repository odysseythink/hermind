package dto

type UploadDocumentResponse struct {
	Documents []any  `json:"documents"`
	Message   string `json:"message,omitempty"`
}

type UpdateEmbeddingsRequest struct {
	Adds    []string `json:"adds"`
	Removes []string `json:"removes"`
}

type UploadLinkRequest struct {
	Link string `json:"link"`
}

type FileMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type MoveFilesResult struct {
	Moved   []string `json:"moved"`
	Skipped []string `json:"skipped"`
}

type MoveFilesRequest struct {
	Files []FileMove `json:"files"`
}
