package dto

import "github.com/odysseythink/hermind/backend/internal/models"

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

type UpdateWorkspaceRequest struct {
	Name                 string   `json:"name,omitempty"`
	OpenAiTemp           *float64 `json:"openAiTemp,omitempty"`
	OpenAiHistory        int      `json:"openAiHistory,omitempty"`
	OpenAiPrompt         *string  `json:"openAiPrompt,omitempty"`
	SimilarityThreshold  *float64 `json:"similarityThreshold,omitempty"`
	ChatProvider         *string  `json:"chatProvider,omitempty"`
	ChatModel            *string  `json:"chatModel,omitempty"`
	TopN                 *int     `json:"topN,omitempty"`
	ChatMode             *string  `json:"chatMode,omitempty"`
	QueryRefusalResponse *string  `json:"queryRefusalResponse,omitempty"`
	// Compression overrides (string-typed to support three-state clearing via FormData)
	CompressEnabled    *string `json:"compressEnabled,omitempty"`    // "true", "false", "default"
	CompressThreshold  *string `json:"compressThreshold,omitempty"`  // "0.75", "", "default"
	CompressContextLen *string `json:"compressContextLen,omitempty"` // "128000", "", "default"
}

type WorkspaceListResponse struct {
	Workspaces []models.Workspace `json:"workspaces"`
}

type CreateThreadRequest struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	ParentThreadID *int   `json:"parentThreadId,omitempty"`
}

type UpdateThreadRequest struct {
	Name string `json:"name"`
}

type CompressRequest struct {
	ThreadID *int   `json:"threadId"`
	Topic    string `json:"topic"`
}
