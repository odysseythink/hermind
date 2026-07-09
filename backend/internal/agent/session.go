package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/mlog"
	pantheonAgent "github.com/odysseythink/pantheon/agent"
	pcompression "github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/conversation"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

// userNoopModel is a TERMINATE-only model for the USER participant.
// It ensures pantheon's runLoop can switch to USER's turn and cleanly exit.
var userNoopModel = &userNoopLanguageModel{}

type userNoopLanguageModel struct{}

func (n *userNoopLanguageModel) Generate(ctx context.Context, req *core.Request) (*core.Response, error) {
	return &core.Response{
		Message: core.Message{Content: core.NewTextContent("TERMINATE")},
	}, nil
}
func (n *userNoopLanguageModel) Stream(ctx context.Context, req *core.Request) (core.StreamResponse, error) {
	return nil, nil
}
func (n *userNoopLanguageModel) GenerateObject(ctx context.Context, req *core.ObjectRequest) (*core.ObjectResponse, error) {
	return nil, nil
}
func (n *userNoopLanguageModel) Provider() string { return "noop" }
func (n *userNoopLanguageModel) Model() string    { return "noop" }

const (
	participantUser  = "USER"
	participantAgent = "@agent"
	defaultMaxRounds = 50
)

type Session struct {
	UUID        string
	WorkspaceID int
	UserID      *int

	conv         *conversation.Conversation
	lm           core.LanguageModel
	systemPrompt string
	pAgent       *pantheonAgent.Agent

	io         AgentIO
	ctx        context.Context
	cancel     context.CancelFunc
	feedbackCh chan feedbackMsg
	terminated chan struct{}
	muteUser   bool

	startedAt time.Time
	once      sync.Once

	// PR-AR-5: approval registry
	approvalsMu sync.Mutex
	approvals   map[string]chan approvalResp
	autoApprove atomic.Bool
	approvalTTL time.Duration

	// PR-AR-5: telemetry
	eventLog eventLogger

	// Context-compression engine (nil when disabled).
	compressor agentcompression.ContextEngine

	currentMessageUUID string
	currentMessageMu   sync.RWMutex
}

// eventLogger is the narrow interface needed for telemetry.
type eventLogger interface {
	LogEvent(ctx context.Context, event string, metadata map[string]any, userID *int) error
}

type feedbackMsg struct {
	Content     string
	Attachments []any
}

func newSession(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO, approvalTTL time.Duration, eventLog eventLogger, compressor agentcompression.ContextEngine) *Session {

	ctx, cancel := context.WithCancel(parentCtx)
	s := &Session{
		UUID:         uuid,
		WorkspaceID:  ws.ID,
		lm:           lm,
		systemPrompt: systemPrompt,
		io:           io,
		ctx:          ctx,
		cancel:       cancel,
		feedbackCh:   make(chan feedbackMsg, 1),
		terminated:   make(chan struct{}),
		muteUser:     true,
		startedAt:    time.Now(),
		approvals:    make(map[string]chan approvalResp),
		approvalTTL:  approvalTTL,
		eventLog:     eventLog,
		compressor:   compressor,
	}
	if user != nil {
		s.UserID = &user.ID
	}
	s.conv = conversation.New(conversation.WithMaxRounds(defaultMaxRounds))
	s.conv.RegisterParticipant(&conversation.Participant{
		Name:      participantUser,
		Role:      "I am the human user.",
		Model:     userNoopModel,
		Interrupt: conversation.InterruptNever,
	})

	s.initAgent(lm, reg)
	installEventBridges(s)
	return s
}

// initAgent builds the pantheon Agent with the current registry and optional
// compressor, then registers it as the @agent participant.
func (s *Session) initAgent(lm core.LanguageModel, reg *tool.Registry) {
	opts := []pantheonAgent.Option{
		pantheonAgent.WithRegistry(reg),
		pantheonAgent.WithMaxSteps(10),
	}
	if s.compressor != nil {
		if obs, ok := s.compressor.(*agentcompression.Observer); ok {
			if inner := obs.Inner(); inner != nil {
				opts = append(opts, pantheonAgent.WithCompressor(inner))
			}
		} else if pc, ok := s.compressor.(*pcompression.Compressor); ok {
			opts = append(opts, pantheonAgent.WithCompressor(pc))
		}
	}
	s.pAgent = pantheonAgent.New(lm, opts...)
	s.conv.RegisterParticipant(&conversation.Participant{
		Name:  participantAgent,
		Role:  s.systemPrompt,
		Agent: s.pAgent,
	})
}

// NewSessionForTesting creates a Session for unit tests. Test-only.
func NewSessionForTesting(parentCtx context.Context, uuid string, ws *models.Workspace, user *models.User,
	lm core.LanguageModel, systemPrompt string, reg *tool.Registry, io AgentIO, compressor agentcompression.ContextEngine) *Session {
	return newSession(parentCtx, uuid, ws, user, lm, systemPrompt, reg, io, 2*time.Minute, nil, compressor)
}

// PantheonAgent returns the underlying pantheon agent for test introspection. Test-only.
func (s *Session) PantheonAgent() *pantheonAgent.Agent {
	return s.pAgent
}

// CompressorForTesting returns the compressor for test introspection. Test-only.
func (s *Session) CompressorForTesting() agentcompression.ContextEngine {
	return s.compressor
}

func (s *Session) Run(prompt string) error {
	err := s.conv.Start(s.ctx, participantUser, participantAgent, prompt)
	s.once.Do(func() { close(s.terminated) })
	return err
}

func (s *Session) Continue(feedback string, attachments []any) {
	select {
	case s.feedbackCh <- feedbackMsg{Content: feedback, Attachments: attachments}:
	default:
		mlog.Warning("agent: dropped feedback (channel full)")
	}
}

func (s *Session) Abort(reason string) {
	if reason != "" {
		_ = s.io.Send(ServerFrame{Type: FrameWSSFailure, Content: reason})
	}
	s.cancelAllApprovals(reason)
	s.cancel()
}

func (s *Session) CurrentMessageUUID() string {
	s.currentMessageMu.RLock()
	defer s.currentMessageMu.RUnlock()
	return s.currentMessageUUID
}

func (s *Session) SetCurrentMessageUUID(uuid string) {
	s.currentMessageMu.Lock()
	defer s.currentMessageMu.Unlock()
	s.currentMessageUUID = uuid
}

var ErrSessionTerminated = errors.New("session terminated")
