package dto

// ---------- OpenAI compatible ----------

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// OpenAIMessage.Content is `any` because OpenAI accepts either a string or an
// array of content parts (text + image_url). Handler-side parsing in PR5
// converts both forms to plain text + attachments.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type OpenAIEmbeddingRequest struct {
	Model string `json:"model"`
	// Input is any because OpenAI accepts string or []string. Handler normalizes.
	Input any `json:"input"`
}

// ---------- API v1 specific request bodies ----------

type APIDocumentUploadRequest struct {
	AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited slugs (Node parity)
	Metadata        any    `json:"metadata,omitempty"`
}

type APIRawTextRequest struct {
	Text            string `json:"textContent"`
	Title           string `json:"title"`
	Metadata        any    `json:"metadata,omitempty"`
	AddToWorkspaces string `json:"addToWorkspaces"` // comma-delimited
}

type APIDocumentRemoveFolderRequest struct {
	Name string `json:"name"`
}

type APISystemRemoveDocumentsRequest struct {
	Names []string `json:"names"`
}

type APIUpdatePinRequest struct {
	DocPath  string `json:"docPath"`
	PinValue bool   `json:"pinStatus"`
}

type APIAdminPreferencesRequest map[string]any
