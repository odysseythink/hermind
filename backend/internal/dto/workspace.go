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
}

type WorkspaceListResponse struct {
	Workspaces []models.Workspace `json:"workspaces"`
}

type CreateThreadRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type UpdateThreadRequest struct {
	Name string `json:"name"`
}
