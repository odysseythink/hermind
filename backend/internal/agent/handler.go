package agent

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/agent/tools"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	pantheonAgent "github.com/odysseythink/pantheon/agent"
	"github.com/odysseythink/pantheon/conversation"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func (r *Runtime) HandleWS(c *gin.Context) {
	id := c.Param("uuid")
	if id == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	inv, err := r.GetInvocation(c.Request.Context(), id)
	if err != nil {
		mlog.Warning("agent: invocation lookup failed: ", id, " err=", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// Resolve workspace + user
	var ws models.Workspace
	if err := r.deps.DB.WithContext(c.Request.Context()).First(&ws, inv.WorkspaceID).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	var user *models.User
	if u, ok := c.Get("user"); ok {
		if uu, ok := u.(*models.User); ok {
			user = uu
		}
	}

	// Resolve LLM before upgrade — if config is broken, return 503 with JSON
	var settings map[string]string
	if r.deps.SysSvc != nil {
		settings, _ = r.deps.SysSvc.GetAllSettings(c.Request.Context())
	}
	lm, err := r.LanguageModelFor(&ws, settings)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "agent: " + err.Error()})
		return
	}
	systemPrompt := resolveSystemPrompt(&ws, user)

	conn, err := r.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		mlog.Error("agent: ws upgrade failed: ", err)
		return
	}
	wc := newWSConn(conn)
	ttl := r.deps.Cfg.AgentToolApprovalTimeout
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}

	// Bound the session with a total lifetime cap.
	sessTTL := r.deps.Cfg.AgentSessionMaxDuration
	if sessTTL <= 0 {
		sessTTL = 30 * time.Minute
	}
	sessCtx, sessCancel := context.WithTimeout(c.Request.Context(), sessTTL)
	defer sessCancel()

	// Two-step construction breaks the circular dep between Session (which owns
	// RequestApproval) and the tool Registry (which needs an ApprovalFn).
	// Step 1: create Session with an empty placeholder registry.
	sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog)

	// Step 2: build the real registry with the session's approval gate.
	reg, err := buildSessionRegistry(c.Request.Context(), r.deps, &ws, user, lm, settings, nil, sess.RequestApproval)
	if err != nil {
		_ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
		return
	}

	// Step 3: replace the placeholder agent with one backed by the real registry.
	sess.pAgent = pantheonAgent.New(lm,
		pantheonAgent.WithRegistry(reg),
		pantheonAgent.WithMaxSteps(10),
	)
	sess.conv.RegisterParticipant(&conversation.Participant{
		Name:  participantAgent,
		Role:  systemPrompt,
		Agent: sess.pAgent,
	})

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
		wc.Close()
	}()

	_ = wc.Send(ServerFrame{Type: FrameStatusResponse, Content: "@agent runtime ready"})

	// reader is a separate goroutine; main goroutine runs the conversation
	go sess.readerLoopWithInput(&wsInput{conn: wc})
	runErr = sess.Run(inv.Prompt)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		content := runErr.Error()
		// Pantheon may wrap context errors; check both the error chain and the string.
		if errors.Is(runErr, context.DeadlineExceeded) || content == "context deadline exceeded" {
			content = "Session reached maximum duration (" + sessTTL.String() + "). Ending now."
		}
		_ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: content})
		return
	}
	// Wait for session to fully terminate (readerLoop may still be active for Continue)
	select {
	case <-sess.terminated:
	case <-sess.ctx.Done():
	}
}

// buildSessionRegistry composes a tool.Registry for the given session.
// The emit function sends status frames to the client; it may be nil in tests.
// approval may be nil, in which case all tools bypass the approval gate.
func buildSessionRegistry(ctx context.Context, deps Deps, ws *models.Workspace, user *models.User, lm core.LanguageModel, settings map[string]string, emit tools.StatusEmitter, approval tools.ApprovalFn) (*tool.Registry, error) {
	if emit == nil {
		emit = func(string) {}
	}
	bd := tools.BuilderDeps{
		VectorSearchSvc: deps.VectorSearchSvc,
		DocSvc:          deps.DocSvc,
		FlowSvc:         deps.FlowSvc,
		FlowExecutor:    deps.FlowExecutor,
		EventLog:        deps.EventLog,
		SysSvc:          deps.SysSvc,
		LM:              lm,
		Approval:        approval,
		Cfg:             deps.Cfg,
		Bridge:          deps.Bridge,
		OutlookOAuth:    deps.OutlookOAuth,
		OutlookStore:    deps.OutlookStore,
		WhitelistSvc:    deps.WhitelistSvc,
		ChatSearcher:    deps.ChatSearcher,
	}
	// Avoid assigning a nil concrete pointer to an interface (Go nil-interface trap).
	if deps.MCPHv != nil {
		bd.MCPHv = deps.MCPHv
	}
	builder := tools.NewBuilder(bd)
	return builder.Build(ctx, ws, user, emit, settings)
}
