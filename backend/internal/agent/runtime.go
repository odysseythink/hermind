package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/odysseythink/hermind/backend/internal/agent/flow"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/mcp"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
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
}

type Runtime struct {
	deps       Deps
	upgrader   websocket.Upgrader
	sessions   sync.Map           // uuid → *Session
	lmCache    sync.Map           // string("provider:model") → core.LanguageModel
	lmOverride core.LanguageModel // test-only
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

// LanguageModelFor returns a LanguageModel for the given workspace, using a
// per-provider+model cache. Tests may bypass the factory via SetTestLanguageModelOverride.
func (r *Runtime) LanguageModelFor(ws *models.Workspace, settings map[string]string) (core.LanguageModel, error) {
	if r.lmOverride != nil {
		return r.lmOverride, nil
	}
	provider := pick("LLMProvider", settings, r.deps.Cfg.LLMProvider)
	model := resolveModelID(provider, ws, settings, r.deps.Cfg)
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
