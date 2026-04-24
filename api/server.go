package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
)

//go:embed webroot/*
var webroot embed.FS

// EngineDeps bundles the providers, storage, and tool registry the
// single-conversation engine needs. cli/engine_deps.go builds this and
// passes it to ServerOpts.
type EngineDeps struct {
	Provider    provider.Provider
	AuxProvider provider.Provider
	Storage     storage.Storage
	ToolReg     *tool.Registry
	SkillsReg   *skills.Registry
	AgentCfg    config.AgentConfig
	Platform    string
	// SkillsEvolver, if non-nil, extracts skills after each conversation.
	SkillsEvolver interface {
		Extract(ctx context.Context, turns []message.Message, verdict *agent.Verdict) error
	}
	// SkillsRetriever, if non-nil, retrieves relevant skills per turn.
	SkillsRetriever interface {
		Retrieve(ctx context.Context, query string, k int) ([]string, error)
	}
	// MemProvider, if non-nil, is added to the engine's MemoryManager so
	// SyncTurn is called after each conversation turn.
	MemProvider memprovider.Provider
}

// ServerOpts bundles server-wide state.
type ServerOpts struct {
	// Config is the live config the server reflects. Required.
	Config *config.Config

	// ConfigPath is where PUT /api/config writes back to. When empty,
	// PUT returns 501.
	ConfigPath string

	// Storage is the backing store for conversation messages. May be nil
	// for meta-only test servers; storage-backed endpoints return 503
	// in that case.
	Storage storage.Storage

	// InstanceRoot is the absolute path to this hermind instance's root
	// directory. Surfaced via GET /api/status so the UI can display it.
	InstanceRoot string

	// Version stamps GET /api/status.
	Version string

	// Streams is the hook the SSE stream subscribes to. Nil means no
	// streaming is available; the hub helper on the server returns a
	// no-op that accepts and drops events.
	Streams StreamHub

	// Deps is the pre-built Engine dependency bundle. Callers
	// (cli/web.go) fill this via cli.BuildEngineDeps. Required for the
	// POST /api/conversation/messages endpoint; zero-value leaves the
	// endpoint returning 503.
	Deps EngineDeps
}

// Server is the API server.
type Server struct {
	opts     *ServerOpts
	router   chi.Router
	bootedAt time.Time
	streams  StreamHub

	// runMu serializes conversation turns — at most one in flight at a time.
	runMu    sync.Mutex
	runCancel context.CancelFunc
}

// NewServer wires routes and middleware.
func NewServer(opts *ServerOpts) (*Server, error) {
	if opts == nil || opts.Config == nil {
		return nil, fmt.Errorf("api: ServerOpts.Config is required")
	}
	streams := opts.Streams
	if streams == nil {
		streams = NewMemoryStreamHub()
	}
	s := &Server{opts: opts, bootedAt: time.Now(), streams: streams}
	s.router = s.buildRouter()
	return s, nil
}

// Router returns the configured chi router.
func (s *Server) Router() chi.Router { return s.router }

// Streams exposes the StreamHub.
func (s *Server) Streams() StreamHub { return s.streams }

// ListenAndServe binds to addr and serves until the server is shut down.
func (s *Server) ListenAndServe(addr string) error {
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return httpSrv.ListenAndServe()
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Get("/model/info", s.handleModelInfo)

		r.Get("/config", s.handleConfigGet)
		r.Put("/config", s.handleConfigPut)
		r.Get("/config/schema", s.handleConfigSchema)

		r.Get("/conversation", s.handleConversationGet)
		r.Post("/conversation/messages", s.handleConversationPost)
		r.Post("/conversation/cancel", s.handleConversationCancel)

		r.Get("/sse", s.handleSSE)

		r.Get("/tools", s.handleToolsList)
		r.Get("/skills", s.handleSkillsList)
		r.Get("/providers", s.handleProvidersList)
		r.Post("/providers/{name}/models", s.handleProvidersModels)
		r.Post("/fallback_providers/{index}/models", s.handleFallbackProvidersModels)
	})

	r.Get("/", s.handleIndex)
	r.Get("/ui/*", s.handleStatic)

	return r
}

func (s *Server) driverName() string {
	if s.opts.Storage == nil {
		return "none"
	}
	if s.opts.Config.Storage.Driver == "" {
		return "sqlite"
	}
	return s.opts.Config.Storage.Driver
}

// handleIndex serves the embedded landing page.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(webroot, "webroot/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// handleStatic serves anything under /ui/* from the embedded webroot.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(webroot, "webroot")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/ui/", http.FileServer(http.FS(sub))).ServeHTTP(w, r)
}

// writeJSON is the single entry point for JSON responses.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONStatus is like writeJSON but sets a non-200 status code first.
func writeJSONStatus(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// RunTurn implements gateway.Runner. It runs one engine turn synchronously
// and returns the assistant reply text. Returns an error if a turn is
// already in flight.
func (s *Server) RunTurn(ctx context.Context, userMessage string) (string, error) {
	if s.opts.Deps.Provider == nil {
		return "", errors.New("provider not configured")
	}
	s.runMu.Lock()
	if s.runCancel != nil {
		s.runMu.Unlock()
		return "", errors.New("another turn is in flight")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.runCancel = cancel
	s.runMu.Unlock()

	defer func() {
		s.runMu.Lock()
		s.runCancel = nil
		s.runMu.Unlock()
		cancel()
	}()

	eng := agent.NewEngineWithToolsAndAux(
		s.opts.Deps.Provider, s.opts.Deps.AuxProvider, s.opts.Deps.Storage,
		s.opts.Deps.ToolReg, s.opts.Deps.AgentCfg, s.opts.Deps.Platform,
	)
	if s.opts.Deps.MemProvider != nil {
		eng.Memory().AddProvider(s.opts.Deps.MemProvider)
		if r, ok := s.opts.Deps.MemProvider.(memprovider.Recaller); ok {
			mc := s.opts.Config.Memory.MetaClaw
			memK := mc.InjectCount
			if memK <= 0 {
				memK = 3
			}
			eng.SetActiveMemoriesProvider(func(ctx context.Context, userMsg string) []memprovider.InjectedMemory {
				out, _ := r.Recall(ctx, userMsg, memK)
				return out
			})
			if mc.BufferEvery > 0 {
				eng.SetBufferEvery(mc.BufferEvery)
			}
			if mc.SynergyTokenBudget > 0 {
				eng.SetSynergyBudget(agent.SynergyBudget{
					TokenBudget:  mc.SynergyTokenBudget,
					SkillRatio:   mc.SynergySkillRatio,
					DedupJaccard: 0.5,
				})
			}
			if mc.JudgeEnabled && s.opts.Deps.AuxProvider != nil {
				eng.SetConversationJudge(agent.NewLLMJudge(s.opts.Deps.AuxProvider))
			}
		}
	}
	wireEngineToHub(eng, s.streams)

	// Wire skills evolver and retriever
	if s.opts.Deps.SkillsEvolver != nil {
		eng.SetSkillsEvolver(s.opts.Deps.SkillsEvolver)
	}
	if s.opts.Deps.SkillsRetriever != nil {
		injectCount := s.opts.Config.Skills.InjectCount
		if injectCount <= 0 {
			injectCount = 3
		}
		ret := s.opts.Deps.SkillsRetriever
		eng.SetActiveSkillsProvider(func(userMsg string) []agent.ActiveSkill {
			snippets, _ := ret.Retrieve(runCtx, userMsg, injectCount)
			return snippetsToActiveSkills(snippets)
		})
	}

	result, err := eng.RunConversation(runCtx, &agent.RunOptions{
		UserMessage: userMessage,
	})
	if err != nil {
		s.streams.Publish(StreamEvent{
			Type: EventTypeError,
			Data: map[string]any{"message": err.Error()},
		})
		return "", err
	}
	s.streams.Publish(StreamEvent{Type: EventTypeDone})
	return result.Response.Content.Text(), nil
}

// StartGateway starts the gateway pump in the background if any platforms
// are configured and enabled. The pump runs until ctx is cancelled.
func (s *Server) StartGateway(ctx context.Context) {
	pump, err := gateway.NewPump(s.opts.Config.Gateway, s)
	if err != nil {
		slog.Error("gateway: startup failed", "err", err)
		return
	}
	if !pump.HasPlatforms() {
		slog.Info("gateway: no enabled platforms, not starting")
		return
	}
	go pump.Start(ctx)
	slog.Info("gateway: pump started")
}

// snippetsToActiveSkills converts a list of skill snippets to ActiveSkill structs.
func snippetsToActiveSkills(snippets []string) []agent.ActiveSkill {
	out := make([]agent.ActiveSkill, 0, len(snippets))
	for _, s := range snippets {
		out = append(out, agent.ActiveSkill{Body: s})
	}
	return out
}
