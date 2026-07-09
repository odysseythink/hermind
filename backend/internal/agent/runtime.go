package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/agent/tools"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/services"
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
	"gorm.io/gorm"
)

// Deps holds the runtime's service dependencies.
type Deps struct {
	DB              *gorm.DB
	Cfg             *config.Config
	TempTokenSvc    *services.TemporaryAuthTokenService
	AuthSvc         *services.AuthService
	SysSvc          *services.SystemService
	VectorSearchSvc *services.VectorSearchService
	DocSvc          *services.DocumentService
	MCPHv           *mcp.Hypervisor
	FlowSvc         *services.AgentFlowService
	FlowExecutor    *flow.Executor
	EventLog        *services.EventLogService
	Bridge          *oauth.BridgeClient
	OutlookOAuth    *oauth.OutlookOAuth
	OutlookStore    *oauth.TokenStore
	WhitelistSvc    *services.AgentSkillWhitelistService
	ChatSearcher    tools.ChatSearcher
	AgentSkillSvc   services.AgentSkillManager
	ProvenanceSvc   services.ProvenanceRecorder
}

type Runtime struct {
	deps       Deps
	upgrader   websocket.Upgrader
	sessions   sync.Map           // uuid → *Session
	lmCache    sync.Map           // string("provider:model") → core.LanguageModel
	lmOverride core.LanguageModel // test-only

	testCompressorOverride agentcompression.ContextEngine // test-only
}

func NewRuntime(d Deps) *Runtime {
	return &Runtime{
		deps: d,
		upgrader: websocket.Upgrader{
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
			HandshakeTimeout: 10 * time.Second,
			CheckOrigin:      buildCheckOrigin(d.Cfg),
		},
	}
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	r.sessions.Range(func(key, value any) bool {
		if s, ok := value.(*Session); ok {
			s.cancelAllApprovals("server shutting down")
			s.Abort("server shutting down")
		}
		return true
	})
	deadline, hasDeadline := ctx.Deadline()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		done := true
		r.sessions.Range(func(_, _ any) bool { done = false; return false })
		if done {
			return nil
		}
		if hasDeadline && time.Now().After(deadline) {
			return fmt.Errorf("agent shutdown: timeout with active sessions")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Sessions returns the internal session map for testing.
func (r *Runtime) Sessions() *sync.Map {
	return &r.sessions
}

// SetTestLanguageModelOverride installs a fixed LanguageModel that bypasses
// the cache and the buildLanguageModel factory. Test-only.
func (r *Runtime) SetTestLanguageModelOverride(m core.LanguageModel) {
	r.lmOverride = m
}

// SetTestCompressorOverride installs a fixed compressor for integration tests.
// Test-only.
func (r *Runtime) SetTestCompressorOverride(c agentcompression.ContextEngine) {
	r.testCompressorOverride = c
}

// SetChatSearcher injects the chat search service after runtime creation.
// Required because ChatService depends on the runtime (AgentInvoker) and
// the runtime needs ChatService for the session-search tool.
func (r *Runtime) SetChatSearcher(cs tools.ChatSearcher) {
	r.deps.ChatSearcher = cs
}

// LanguageModelFor returns a LanguageModel for the given workspace, using a
// per-provider+model cache. Tests may bypass the factory via SetTestLanguageModelOverride.
func (r *Runtime) LanguageModelFor(ws *models.Workspace, settings map[string]string) (core.LanguageModel, error) {
	if r.lmOverride != nil {
		return r.lmOverride, nil
	}
	provider := pick("LLMProvider", settings, r.deps.Cfg.LLMProvider)
	model := providers.ResolveModelID(provider, r.deps.Cfg, settings)
	key := provider + ":" + model
	if cached, ok := r.lmCache.Load(key); ok {
		return cached.(core.LanguageModel), nil
	}
	lm, err := buildLanguageModel(ws, settings, r.deps.Cfg)
	if err != nil {
		return nil, err
	}
	r.lmCache.Store(key, lm)
	return lm, nil
}

// RunAgentDirectly is the non-WS entrypoint for Telegram (and future Discord/Slack).
func (r *Runtime) RunAgentDirectly(ctx context.Context, invUUID string, io AgentIO, input AgentInput) error {
	inv, err := r.GetInvocation(ctx, invUUID)
	if err != nil {
		return err
	}

	var ws models.Workspace
	if err := r.deps.DB.WithContext(ctx).First(&ws, inv.WorkspaceID).Error; err != nil {
		return err
	}
	var user *models.User
	if inv.UserID != nil {
		user = &models.User{ID: *inv.UserID}
	}

	var settings map[string]string
	if r.deps.SysSvc != nil {
		settings, _ = r.deps.SysSvc.GetAllSettings(ctx)
	}
	lm, err := r.LanguageModelFor(&ws, settings)
	if err != nil {
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: "agent: " + err.Error()})
		return err
	}
	var skills []models.AgentSkill
	if r.deps.AgentSkillSvc != nil {
		skills, _ = r.deps.AgentSkillSvc.ListActiveByWorkspace(ctx, ws.ID)
	}
	systemPrompt := resolveSystemPrompt(&ws, user, skills)

	ttl := r.deps.Cfg.AgentToolApprovalTimeout
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	sessTTL := r.deps.Cfg.AgentSessionMaxDuration
	if sessTTL <= 0 {
		sessTTL = 30 * time.Minute
	}
	sessCtx, sessCancel := context.WithTimeout(ctx, sessTTL)
	defer sessCancel()

	var comp agentcompression.ContextEngine
	if r.testCompressorOverride != nil {
		comp = r.testCompressorOverride
	} else {
		comp = buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc, nil)
	}
	sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), io, ttl, r.deps.EventLog, comp)

	citationEmitter := func(citations []tools.Citation) {
		uuid := sess.CurrentMessageUUID()
		if uuid == "" || len(citations) == 0 {
			return
		}
		_ = io.Send(ServerFrame{
			Type: FrameReportStreamEvent,
			ContentObj: map[string]any{
				"type":      "citations",
				"uuid":      uuid,
				"citations": citations,
			},
		})
	}

	reg, err := buildSessionRegistry(ctx, r.deps, &ws, user, lm, settings, nil, sess.RequestApproval, citationEmitter)
	if err != nil {
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
		return err
	}
	sess.initAgent(lm, reg)

	r.sessions.Store(inv.UUID, sess)
	var userID *int
	if user != nil {
		userID = &user.ID
	}
	logChatStarted(r.deps.EventLog, userID, inv.UUID, ws.ID, lm.Provider(), lm.Model())

	var runErr error
	defer func() {
		duration := time.Since(sess.startedAt)
		reason := "normal"
		if runErr != nil {
			if errors.Is(runErr, context.DeadlineExceeded) {
				reason = "timeout"
			} else if errors.Is(runErr, context.Canceled) {
				reason = "cancelled"
			} else {
				reason = "error"
			}
		}
		logChatTerminated(r.deps.EventLog, userID, inv.UUID, reason, duration)
		r.sessions.Delete(inv.UUID)
		_ = r.CloseInvocation(context.Background(), inv.UUID)
		io.Close()
	}()

	_ = io.Send(ServerFrame{Type: FrameStatusResponse, Content: "@agent runtime ready"})

	go sess.readerLoopWithInput(input)
	runErr = sess.Run(inv.Prompt)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		content := runErr.Error()
		if errors.Is(runErr, context.DeadlineExceeded) || content == "context deadline exceeded" {
			content = "Session reached maximum duration (" + sessTTL.String() + "). Ending now."
		}
		_ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: content})
	}
	select {
	case <-sess.terminated:
	case <-sess.ctx.Done():
	}
	return runErr
}
