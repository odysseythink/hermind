package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/collector"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

// ApprovalFn is the signature for the per-session tool approval gate.
// When nil, all tools bypass approval.
type ApprovalFn func(ctx context.Context, skillName string, args any, description string) (approved bool, reason string)

// BuilderDeps holds the service dependencies needed by the Builder.
// Kept separate from agent.Deps to avoid an import cycle.
type BuilderDeps struct {
	VectorSearchSvc VectorSearcher
	DocSvc          DocumentLister
	MCPHv           MCPHypervisor
	FlowSvc         *services.AgentFlowService
	FlowExecutor    *flow.Executor
	EventLog        EventLogger
	SysSvc          *services.SystemService
	LM              core.LanguageModel
	Approval        ApprovalFn     // nil = always approve (test default)
	Cfg             *config.Config // static config for skill enablement / paths
	Bridge          *oauth.BridgeClient
	OutlookOAuth    *oauth.OutlookOAuth
	OutlookStore    *oauth.TokenStore
	Collector       *collector.Client // nil = attachment parsing unavailable
	WhitelistSvc    *services.AgentSkillWhitelistService
	ChatSearcher    ChatSearcher
	AgentSkillSvc   services.AgentSkillManager
	ProvenanceSvc   services.ProvenanceRecorder
}

// Builder composes a tool.Registry from multiple sources per session.
type Builder struct {
	deps BuilderDeps
}

// NewBuilder creates a Builder from the given dependencies.
func NewBuilder(deps BuilderDeps) *Builder {
	return &Builder{deps: deps}
}

// Build composes the registry for a single session.
// Sources are merged in priority order; later sources override earlier ones.
func (b *Builder) Build(ctx context.Context, ws *models.Workspace, user *models.User, emit StatusEmitter, settings map[string]string) (*tool.Registry, error) {
	reg := tool.NewRegistry()
	disabled := parseDisabledSkills(settings["disabled_agent_skills"])
	seen := make(map[string]string) // name → source
	globalAutoApprove := settings["agent_tool_auto_approve"] == "true"

	var userID *int
	if user != nil {
		userID = &user.ID
	}
	var whitelist []string
	if b.deps.WhitelistSvc != nil {
		whitelist, _ = b.deps.WhitelistSvc.Get(ctx, userID)
	}

	tc := &ToolContext{
		Ctx:             ctx,
		Workspace:       ws,
		User:            user,
		LM:              b.deps.LM,
		Settings:        settings,
		VectorSearchSvc: b.deps.VectorSearchSvc,
		DocSvc:          b.deps.DocSvc,
		MCPHv:           b.deps.MCPHv,
		FlowSvc:         b.deps.FlowSvc,
		EventLog:        b.deps.EventLog,
		Emit:            emit,
		Approval:        b.deps.Approval,
		Cfg:             b.deps.Cfg,
		AgentSkillSvc:   b.deps.AgentSkillSvc,
		ProvenanceSvc:   b.deps.ProvenanceSvc,
	}

	// Source 1: default skills
	for _, e := range []*tool.Entry{
		NewRAGMemorySkill(tc),
		NewDocSummarizerSkill(tc),
		NewWebScrapingSkill(tc),
		NewRechartSkill(tc),
		NewSQLAgentSkill(tc),
		NewFilesystemAgentSkill(tc),
		NewCreateFilesAgentSkill(tc),
		NewGmailAgentSkill(tc, b.deps),
		NewGCalAgentSkill(tc, b.deps),
		NewOutlookAgentSkill(tc, b.deps),
		NewSessionSearchSkill(tc, b.deps.ChatSearcher),
	} {
		if isDisabled(e.Name, disabled) {
			continue
		}
		b.addWithApproval(reg, seen, e, "default", false, globalAutoApprove, whitelist)
	}

	// Meta-skills: skill management tools — always enabled, not subject to disabled_skills list
	if b.deps.AgentSkillSvc != nil {
		for _, e := range []*tool.Entry{
			NewSkillManageSkill(tc, b.deps.AgentSkillSvc, b.deps.ProvenanceSvc),
			NewSkillsListSkill(tc, b.deps.AgentSkillSvc),
			NewSkillViewSkill(tc, b.deps.AgentSkillSvc),
		} {
			b.addWithApproval(reg, seen, e, "default", false, globalAutoApprove, whitelist)
		}
	}

	// Source 2: MCP (requires approval)
	if b.deps.MCPHv != nil {
		for _, qname := range b.deps.MCPHv.ActiveServers() {
			serverName := strings.TrimPrefix(qname, "@@mcp_")
			plugins, err := b.deps.MCPHv.ToolsAsPlugins(serverName)
			if err != nil {
				mlog.Warning("agent: MCP ToolsAsPlugins(", serverName, "): ", err)
				continue
			}
			for _, p := range plugins {
				if isDisabled(p.QualifiedName, disabled) {
					continue
				}
				b.addWithApproval(reg, seen, mcpToolToEntry(p, emit), "mcp:"+serverName, true, globalAutoApprove, whitelist)
			}
		}
	}

	// Source 3: AgentFlow (requires approval)
	if b.deps.FlowSvc != nil {
		flows, err := b.deps.FlowSvc.ListFlows()
		if err == nil {
			for _, f := range flows {
				if !f.Active {
					continue
				}
				e := flowToEntry(f, b.deps.FlowSvc, b.deps.FlowExecutor, emit)
				if isDisabled(e.Name, disabled) {
					continue
				}
				b.addWithApproval(reg, seen, e, "flow:"+f.UUID, true, globalAutoApprove, whitelist)
			}
		}
	}

	// Source 4: imported (v1 stub, no-op)
	return reg, nil
}

// addWithApproval registers an entry, optionally wrapping its Handler with an
// approval gate when requiresApproval is true and auto-approve is off.
func (b *Builder) addWithApproval(reg *tool.Registry, seen map[string]string, e *tool.Entry, source string, requiresApproval bool, globalAutoApprove bool, whitelist []string) {
	if requiresApproval && !globalAutoApprove && !containsStr(whitelist, e.Name) && b.deps.Approval != nil {
		inner := e.Handler
		e.Handler = func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args any
			_ = json.Unmarshal(raw, &args)
			approved, reason := b.deps.Approval(ctx, e.Name, args, e.Description)
			if !approved {
				return tool.Error("Tool execution rejected: " + reason), nil
			}
			return inner(ctx, raw)
		}
	}
	if prior, ok := seen[e.Name]; ok {
		if b.deps.EventLog != nil {
			_ = b.deps.EventLog.LogEvent(context.Background(), "agent.tool.override",
				map[string]any{"tool": e.Name, "from": prior, "to": source}, nil)
		}
	}
	seen[e.Name] = source
	reg.Register(e)
}

// parseDisabledSkills parses a JSON array of disabled tool names.
// Tolerates absent, empty, null, or malformed input.
func parseDisabledSkills(raw string) []string {
	if raw == "" || raw == "null" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

func isDisabled(name string, disabled []string) bool {
	for _, d := range disabled {
		if d == name {
			return true
		}
	}
	return false
}

func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// buildSchema creates a core.Schema from raw JSON schema bytes.
func buildSchema(raw json.RawMessage) *core.Schema {
	if len(raw) == 0 {
		return nil
	}
	s, err := core.SchemaFromJSON(raw)
	if err != nil {
		return nil
	}
	return s
}
