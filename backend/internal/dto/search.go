package dto

type SearchResults struct {
	Workspaces []WorkspaceSearchResult `json:"workspaces"`
	Threads    []ThreadSearchResult    `json:"threads"`
}

type WorkspaceSearchResult struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type ThreadSearchResult struct {
	Slug      string                `json:"slug"`
	Name      string                `json:"name"`
	Workspace WorkspaceSearchResult `json:"workspace"`
}
