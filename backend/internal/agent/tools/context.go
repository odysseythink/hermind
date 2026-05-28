// Package tools provides the default agent skills, MCP/AgentFlow projection
// adapters, and the Registry Builder for the Hermind agent runtime.
package tools

import (
	"context"

	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
)

// StatusEmitter sends a progress/status message to the WebSocket client.
type StatusEmitter func(msg string)

// EventLogger is the subset of EventLogService needed by tools.
type EventLogger interface {
	LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error
}

// MCPHypervisor is the subset of mcp.Hypervisor needed by tools.
type MCPHypervisor interface {
	ActiveServers() []string
	ToolsAsPlugins(name string) ([]mcp.ToolPlugin, error)
}

// VectorSearcher is the subset of VectorSearchService needed by tools.
type VectorSearcher interface {
	Search(ctx context.Context, ws *models.Workspace, req dto.VectorSearchRequest) ([]dto.VectorSearchResult, error)
}

// DocumentLister is the subset of DocumentService needed by tools.
type DocumentLister interface {
	ListDocuments(ctx context.Context, folder string) ([]models.WorkspaceDocument, error)
}

// ToolContext holds all per-session dependencies passed to each skill factory.
type ToolContext struct {
	Ctx             context.Context
	Workspace       *models.Workspace
	User            *models.User
	Settings        map[string]string
	LM              core.LanguageModel
	VectorSearchSvc VectorSearcher
	DocSvc          DocumentLister
	MCPHv           MCPHypervisor
	FlowSvc         *services.AgentFlowService
	EventLog        EventLogger
	Emit            StatusEmitter
	Approval        ApprovalFn // nil = no gate (test default)
	Cfg             *config.Config
}
