package tools

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuilder_RegistersDefaultSkills(t *testing.T) {
	b := NewBuilder(BuilderDeps{})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	// rag-memory, document-summarizer, sql-agent, filesystem-agent, create-files-agent
	// are filtered by CheckFn when services/config are nil.
	// web-browsing and session-search have no CheckFn so they always appear.
	require.Len(t, entries, 4)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.False(t, names["rag-memory"])
	require.False(t, names["document-summarizer"])
	require.True(t, names["web-scraping"])
	require.True(t, names["web-browsing"])
	require.True(t, names["rechart"])
	require.True(t, names["session-search"])
	require.False(t, names["sql-agent"])
	require.False(t, names["filesystem-agent"])
	require.False(t, names["create-files-agent"])
}

func TestBuilder_RespectsDisabledFilter(t *testing.T) {
	b := NewBuilder(BuilderDeps{})
	settings := map[string]string{"disabled_agent_skills": `["rag-memory","web-scraping","web-browsing"]`}
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	// rechart and session-search remain (document-summarizer filtered by CheckFn, rag-memory/web-scraping/web-browsing disabled)
	require.Len(t, entries, 2)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["rechart"])
	require.True(t, names["session-search"])
	require.False(t, names["rag-memory"])
	require.False(t, names["web-scraping"])
	require.False(t, names["web-browsing"])
	require.False(t, names["document-summarizer"])
}

func TestBuilder_DedupLastWins_FiresOverrideEventLog(t *testing.T) {
	var eventCount atomic.Int32
	mockEventLog := &mockEventLog{count: &eventCount}

	b := NewBuilder(BuilderDeps{
		MCPHv:    newMockHypervisor(),
		EventLog: mockEventLog,
	})
	settings := map[string]string{}
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)

	// The mock MCP server exposes a tool named "rag-memory" which collides
	// with the default skill. The MCP entry should win and fire an override event.
	require.Equal(t, int32(1), eventCount.Load(), "expected one override event")

	entries := reg.Entries(nil)
	// document-summarizer is hidden by CheckFn (DocSvc nil).
	require.Len(t, entries, 5)

	var ragEntry *tool.Entry
	for _, e := range entries {
		if e.Name == "rag-memory" {
			ragEntry = e
			break
		}
	}
	require.NotNil(t, ragEntry)
	require.Equal(t, "mcp", ragEntry.Toolset, "dedup should keep the later MCP entry")
}

func TestBuilder_DedupPreservesLater(t *testing.T) {
	b := NewBuilder(BuilderDeps{
		MCPHv: newMockHypervisor(),
	})
	settings := map[string]string{}
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)

	// Dispatch the colliding "rag-memory" tool — should route to the MCP stub
	result, err := reg.Dispatch(context.Background(), "rag-memory", nil)
	require.NoError(t, err)
	require.Contains(t, result, "mcp-stub-result")
}

func TestBuilder_DefaultSkills_NoApprovalWrap(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	b := NewBuilder(BuilderDeps{Approval: approval})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	// web-scraping is a default skill and should NOT consult Approval (tool-level gate is false)
	result, err := reg.Dispatch(context.Background(), "web-scraping", []byte(`{"url":"http://example.com"}`))
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.False(t, called.Load(), "default skill should not trigger approval")
}

func TestBuilder_MCPTools_HaveApprovalWrap(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	b := NewBuilder(BuilderDeps{
		MCPHv:    newMockHypervisor(),
		Approval: approval,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "rag-memory", nil)
	require.NoError(t, err)
	require.Contains(t, result, "mcp-stub-result")
	require.True(t, called.Load(), "MCP tool should trigger approval")
}

func TestBuilder_FlowTools_HaveApprovalWrap(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	// Create a temp flow file
	tmpDir := t.TempDir()
	flowSvc := services.NewAgentFlowService(tmpDir)
	_, err := flowSvc.SaveFlow("Test Flow", services.FlowConfig{
		Description: "A test flow",
		Active:      true,
	}, "flow-123")
	require.NoError(t, err)

	b := NewBuilder(BuilderDeps{
		FlowSvc:  flowSvc,
		Approval: approval,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "flow-test-flow", nil)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.True(t, called.Load(), "Flow tool should trigger approval")
}

func TestBuilder_ApprovalRejects_HandlerNotCalled(t *testing.T) {
	mhv := newMockHypervisor()
	approval := func(context.Context, string, any, string) (bool, string) {
		return false, "rejected by test"
	}

	b := NewBuilder(BuilderDeps{
		MCPHv:    mhv,
		Approval: approval,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "rag-memory", nil)
	require.NoError(t, err)
	require.Contains(t, result, "Tool execution rejected")
	require.Equal(t, int32(0), mhv.callCount.Load(), "inner MCP handler should not be called when approval rejects")
}

func TestBuilder_GlobalAutoApprove_BypassesGate(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	b := NewBuilder(BuilderDeps{
		MCPHv:    newMockHypervisor(),
		Approval: approval,
	})
	settings := map[string]string{"agent_tool_auto_approve": "true"}
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "rag-memory", nil)
	require.NoError(t, err)
	require.Contains(t, result, "mcp-stub-result")
	require.False(t, called.Load(), "global auto-approve should bypass approval gate")
}

func TestBuilder_SQLAgent_RegisteredWhenConnectionsConfigured(t *testing.T) {
	settings := map[string]string{
		"agent_sql_connections": `[{"database_id":"test","engine":"sqlite","connectionString":":memory:"}]`,
	}
	b := NewBuilder(BuilderDeps{})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)
	require.NotNil(t, reg)
	entries := reg.Entries(nil)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["sql-agent"])
}

func TestBuilder_SQLAgent_NotRegisteredWhenNoConnections(t *testing.T) {
	b := NewBuilder(BuilderDeps{})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)
	entries := reg.Entries(nil)
	for _, e := range entries {
		require.NotEqual(t, "sql-agent", e.Name)
	}
}

func TestBuilder_FilesystemAgent_RegisteredWhenEnabled(t *testing.T) {
	b := NewBuilder(BuilderDeps{Cfg: &config.Config{AgentFilesystemEnabled: true, AgentFilesystemRoot: t.TempDir()}})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)
	entries := reg.Entries(nil)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["filesystem-agent"])
}

func TestBuilder_FilesystemAgent_NotRegisteredWhenDisabled(t *testing.T) {
	b := NewBuilder(BuilderDeps{Cfg: &config.Config{AgentFilesystemEnabled: false}})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)
	entries := reg.Entries(nil)
	for _, e := range entries {
		require.NotEqual(t, "filesystem-agent", e.Name)
	}
}

func TestBuilder_CreateFilesAgent_RegisteredWhenEnabled(t *testing.T) {
	b := NewBuilder(BuilderDeps{Cfg: &config.Config{AgentCreateFilesEnabled: true, AgentCreateFilesDir: t.TempDir()}})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)
	entries := reg.Entries(nil)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["create-files-agent"])
}

func TestBuilder_CreateFilesAgent_NotRegisteredWhenDisabled(t *testing.T) {
	b := NewBuilder(BuilderDeps{Cfg: &config.Config{AgentCreateFilesEnabled: false}})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)
	entries := reg.Entries(nil)
	for _, e := range entries {
		require.NotEqual(t, "create-files-agent", e.Name)
	}
}

func TestBuilder_WhitelistedSkill_BypassesApproval(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	tmpDir := t.TempDir()
	flowSvc := services.NewAgentFlowService(tmpDir)
	_, err := flowSvc.SaveFlow("Test Flow", services.FlowConfig{
		Description: "A test flow",
		Active:      true,
	}, "flow-123")
	require.NoError(t, err)

	// Create a mock whitelist service that whitelists the flow tool
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))
	sysSvc := services.NewSystemService(db)
	whitelistSvc := services.NewAgentSkillWhitelistService(sysSvc)
	require.NoError(t, whitelistSvc.Add(context.Background(), nil, "flow-test-flow"))

	b := NewBuilder(BuilderDeps{
		FlowSvc:      flowSvc,
		Approval:     approval,
		WhitelistSvc: whitelistSvc,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "flow-test-flow", nil)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.False(t, called.Load(), "whitelisted skill should bypass approval")
}

func TestBuilder_NonWhitelisted_StillRequiresApproval(t *testing.T) {
	var called atomic.Bool
	approval := func(context.Context, string, any, string) (bool, string) {
		called.Store(true)
		return true, ""
	}

	tmpDir := t.TempDir()
	flowSvc := services.NewAgentFlowService(tmpDir)
	_, err := flowSvc.SaveFlow("Test Flow", services.FlowConfig{
		Description: "A test flow",
		Active:      true,
	}, "flow-123")
	require.NoError(t, err)

	b := NewBuilder(BuilderDeps{
		FlowSvc:  flowSvc,
		Approval: approval,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	result, err := reg.Dispatch(context.Background(), "flow-test-flow", nil)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	require.True(t, called.Load(), "non-whitelisted skill should require approval")
}

func TestBuilder_WiresCitationEmitterToToolContext(t *testing.T) {
	captured := func(citations []Citation) {
		_ = citations
	}
	b := NewBuilder(BuilderDeps{CitationEmitter: captured})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	var found bool
	for _, e := range entries {
		if e.Name == "web-browsing" {
			found = true
			break
		}
	}
	require.True(t, found, "web-browsing should be registered")
}

func TestBuilder_WebBrowsing_RegisteredByDefault(t *testing.T) {
	b := NewBuilder(BuilderDeps{})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, nil)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["web-browsing"], "web-browsing should be registered as a default skill")
}

func TestBuilder_AllSevenDefaultSkills_PresentInRegistry(t *testing.T) {
	settings := map[string]string{
		"agent_sql_connections": `[{"database_id":"test","engine":"sqlite","connectionString":":memory:"}]`,
	}
	b := NewBuilder(BuilderDeps{
		VectorSearchSvc: &mockVectorSearcher{results: nil, err: nil},
		DocSvc:          &mockDocumentLister{docs: nil, err: nil},
		LM:              &mockLM{},
		Cfg: &config.Config{
			AgentFilesystemEnabled:  true,
			AgentFilesystemRoot:     t.TempDir(),
			AgentCreateFilesEnabled: true,
			AgentCreateFilesDir:     t.TempDir(),
		},
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, nil, func(string) {}, settings)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	require.Len(t, entries, 9)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["rag-memory"])
	require.True(t, names["document-summarizer"])
	require.True(t, names["web-scraping"])
	require.True(t, names["web-browsing"])
	require.True(t, names["rechart"])
	require.True(t, names["sql-agent"])
	require.True(t, names["filesystem-agent"])
	require.True(t, names["create-files-agent"])
	require.True(t, names["session-search"])
}

func TestBuilder_AllTenDefaultSkillsRegistered_InSingleUserMode(t *testing.T) {
	settings := map[string]string{
		"agent_sql_connections":        `[{"database_id":"test","engine":"sqlite","connectionString":":memory:"}]`,
		"gmail_agent_config":           `{"deploymentId":"d","apiKey":"k"}`,
		"google_calendar_agent_config": `{"deploymentId":"d","apiKey":"k"}`,
		"outlook_agent_config":         `{"clientId":"c","clientSecret":"s"}`,
	}

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	b := NewBuilder(BuilderDeps{
		VectorSearchSvc: &mockVectorSearcher{results: nil, err: nil},
		DocSvc:          &mockDocumentLister{docs: nil, err: nil},
		LM:              &mockLM{},
		Cfg: &config.Config{
			MultiUserMode:           false,
			AgentFilesystemEnabled:  true,
			AgentFilesystemRoot:     t.TempDir(),
			AgentCreateFilesEnabled: true,
			AgentCreateFilesDir:     t.TempDir(),
		},
		Bridge:       &oauth.BridgeClient{},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, &models.User{ID: 1}, func(string) {}, settings)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	require.Len(t, entries, 12)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["rag-memory"])
	require.True(t, names["document-summarizer"])
	require.True(t, names["web-scraping"])
	require.True(t, names["web-browsing"])
	require.True(t, names["rechart"])
	require.True(t, names["sql-agent"])
	require.True(t, names["filesystem-agent"])
	require.True(t, names["create-files-agent"])
	require.True(t, names["gmail-agent"])
	require.True(t, names["google-calendar-agent"])
	require.True(t, names["outlook-agent"])
	require.True(t, names["session-search"])
}

func TestBuilder_OAuthSkillsVisible_InMultiUserMode_WhenConfigured(t *testing.T) {
	settings := map[string]string{
		"agent_sql_connections":        `[{"database_id":"test","engine":"sqlite","connectionString":":memory:"}]`,
		"gmail_agent_config":           `{"deploymentId":"d","apiKey":"k"}`,
		"google_calendar_agent_config": `{"deploymentId":"d","apiKey":"k"}`,
		"outlook_agent_config":         `{"clientId":"c","clientSecret":"s"}`,
	}

	db, enc := newOutlookTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	outlookOAuth := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	require.NoError(t, store.Save(context.Background(), 1, &oauth.TokenSet{
		AccessToken:  "at",
		RefreshToken: "rt",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	b := NewBuilder(BuilderDeps{
		VectorSearchSvc: &mockVectorSearcher{results: nil, err: nil},
		DocSvc:          &mockDocumentLister{docs: nil, err: nil},
		LM:              &mockLM{},
		Cfg: &config.Config{
			MultiUserMode:           true,
			AgentFilesystemEnabled:  true,
			AgentFilesystemRoot:     t.TempDir(),
			AgentCreateFilesEnabled: true,
			AgentCreateFilesDir:     t.TempDir(),
		},
		Bridge:       &oauth.BridgeClient{},
		OutlookOAuth: outlookOAuth,
		OutlookStore: store,
	})
	reg, err := b.Build(context.Background(), &models.Workspace{ID: 1}, &models.User{ID: 1}, func(string) {}, settings)
	require.NoError(t, err)

	entries := reg.Entries(nil)
	// All 12 skills (9 base + 3 OAuth) should appear when configured.
	require.Len(t, entries, 12)

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	require.True(t, names["gmail-agent"])
	require.True(t, names["google-calendar-agent"])
	require.True(t, names["outlook-agent"])
}

// mockLM satisfies core.LanguageModel for builder tests.
type mockLM struct{}

func (m *mockLM) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return nil, nil
}
func (m *mockLM) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, nil
}
func (m *mockLM) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, nil
}
func (m *mockLM) Provider() string { return "mock" }
func (m *mockLM) Model() string    { return "mock" }

// mockEventLog counts calls to LogEvent.
type mockEventLog struct {
	count *atomic.Int32
}

func (m *mockEventLog) LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error {
	m.count.Add(1)
	return nil
}

// mockHypervisor implements just enough of the mcp.Hypervisor interface for tests.
type mockHypervisor struct {
	servers   []string
	plugins   map[string][]mcp.ToolPlugin
	callCount atomic.Int32
}

func newMockHypervisor() *mockHypervisor {
	mhv := &mockHypervisor{servers: []string{"@@mcp_test"}}
	mhv.plugins = map[string][]mcp.ToolPlugin{
		"test": {
			{
				ServerName:    "test",
				ToolName:      "rag-memory",
				QualifiedName: "rag-memory",
				Description:   "test tool",
				InputSchema:   nil,
				Call: func(ctx context.Context, args map[string]any) (any, error) {
					mhv.callCount.Add(1)
					return map[string]any{"result": "mcp-stub-result"}, nil
				},
			},
		},
	}
	return mhv
}

func (m *mockHypervisor) ActiveServers() []string { return m.servers }
func (m *mockHypervisor) ToolsAsPlugins(name string) ([]mcp.ToolPlugin, error) {
	return m.plugins[name], nil
}
