package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/agent"
	"github.com/odysseythink/hermind/agent/idle"
	"github.com/odysseythink/hermind/agent/presence"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/skills"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/odysseythink/hermind/tool/memory/memprovider"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
)

//go:embed webroot/*
var webroot embed.FS

// EngineDeps bundles the providers, storage, and tool registry the
// single-conversation engine needs. cli/engine_deps.go builds this and
// passes it to ServerOpts.
type EngineDeps struct {
	Provider    core.LanguageModel
	AuxProvider core.LanguageModel
	Storage     storage.Storage
	ToolReg     *tool.Registry
	SkillsReg   *skills.Registry
	AgentCfg    config.AgentConfig
	Platform    string
	// SkillsEvolver, if non-nil, extracts skills after each conversation.
	SkillsEvolver interface {
		Extract(ctx context.Context, turns []message.HermindMessage, verdict *agent.Verdict) error
	}
	// SkillsRetriever, if non-nil, retrieves relevant skills per turn.
	SkillsRetriever interface {
		Retrieve(ctx context.Context, query string, k int) ([]string, error)
	}
	// MemProvider, if non-nil, is added to the engine's MemoryManager so
	// SyncTurn is called after each conversation turn.
	MemProvider memprovider.Provider
	// SkillsTracker maintains the skills-library content hash and
	// generation seq used by the memory ranker to decay stale signals.
	// Constructed at startup; nil if skills are not available.
	SkillsTracker *skills.Tracker
	// HTTPIdle is the presence source that records HTTP activity. The
	// /api/* middleware notes activity on every request so the idle
	// consolidator (and future presence consumers) can detect quiet
	// windows.
	HTTPIdle *presence.HTTPIdle
	// Presence is the composed user-presence Provider used by the idle
	// consolidator and exposed via /api/memory/health for diagnostics.
	Presence presence.Provider
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
	Deps *EngineDeps

	// DepsBuilder rebuilds the provider-dependent parts of EngineDeps
	// from a new config. When non-nil, handleConfigPut invokes it after
	// parsing the payload so model/provider changes take effect without
	// a server restart.
	DepsBuilder func(ctx context.Context, cfg *config.Config, current *EngineDeps) (*EngineDeps, error)
}

// Server is the API server.
type Server struct {
	opts     *ServerOpts
	router   chi.Router
	bootedAt time.Time
	streams  StreamHub

	// runMu serializes conversation turns — at most one in flight at a time.
	runMu     sync.Mutex
	runCancel context.CancelFunc

	idle *idle.IdleConsolidator

	// deps holds the current EngineDeps and is swapped atomically when
	// the configuration is hot-reloaded.
	deps atomic.Pointer[EngineDeps]
}

// NewServer wires routes and middleware.
func NewServer(opts *ServerOpts) (*Server, error) {
	if opts == nil || opts.Config == nil {
		return nil, fmt.Errorf("api: ServerOpts.Config is required")
	}
	if opts.Deps == nil {
		opts.Deps = &EngineDeps{}
	}
	streams := opts.Streams
	if streams == nil {
		streams = NewMemoryStreamHub()
	}
	s := &Server{opts: opts, bootedAt: time.Now(), streams: streams}
	s.deps.Store(opts.Deps)
	s.router = s.buildRouter()
	return s, nil
}

// Router returns the configured chi router.
func (s *Server) Router() chi.Router { return s.router }

// Streams exposes the StreamHub.
func (s *Server) Streams() StreamHub { return s.streams }

// SetIdleConsolidator registers the idle consolidator; server middleware
// will NoteActivity on every request so the consolidator knows the
// instance is busy.
func (s *Server) SetIdleConsolidator(c *idle.IdleConsolidator) {
	s.idle = c
}

// currentDeps returns the live EngineDeps. Callers must not mutate the
// returned value.
func (s *Server) currentDeps() *EngineDeps {
	return s.deps.Load()
}

// disabledTools returns a set of tool names that are currently disabled
// according to the live config. Since s.opts.Config is atomically
// updated by handleConfigPut, this always reflects the latest state.
func (s *Server) disabledTools() map[string]bool {
	m := make(map[string]bool)
	for _, name := range s.opts.Config.Tools.Disabled {
		m[name] = true
	}
	return m
}

// activeToolReg returns a new Registry containing only the tools that
// are NOT in the disabled list. Callers get a fresh copy on each call
// so the result is safe to pass into the engine. The overhead is
// negligible because registries are small (< 50 entries).
func (s *Server) activeToolReg() *tool.Registry {
	s.injectFilesystemConfig()
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		return nil
	}
	disabled := s.disabledTools()
	active := tool.NewRegistry()

	// Check filesystem master toggle
	filesystemDisabled := disabled["filesystem"]

	// Read subtool enablement from settings
	subtoolEnabled := make(map[string]bool)
	if fsSettings, ok := s.opts.Config.Tools.Settings["filesystem"]; ok {
		for key, val := range fsSettings {
			if key == "allowed_directories" {
				continue
			}
			if b, ok := val.(bool); ok {
				subtoolEnabled[key] = b
			}
		}
	}

	for _, e := range deps.ToolReg.Entries(nil) {
		// Virtual filesystem entry has no handler — skip from engine registry
		if e.Name == "filesystem" {
			continue
		}

		// If filesystem is disabled, drop all file toolset entries
		if e.Toolset == "file" && filesystemDisabled {
			continue
		}

		// If filesystem is enabled, check per-subtool setting
		if e.Toolset == "file" && !filesystemDisabled {
			if enabled, ok := subtoolEnabled[e.Name]; ok && !enabled {
				continue
			}
		}

		// Global disabled list check
		if disabled[e.Name] {
			continue
		}

		active.Register(e)
	}
	return active
}

// injectFilesystemConfig sets the current filesystem configuration
// so that file tool handlers can access allowed_directories and subtool settings.
func (s *Server) injectFilesystemConfig() {
	cfg := map[string]any{}
	if fsSettings, ok := s.opts.Config.Tools.Settings["filesystem"]; ok {
		for k, v := range fsSettings {
			cfg[k] = v
		}
	}
	file.SetCurrentConfig(cfg)
}

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

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if h := s.currentDeps().HTTPIdle; h != nil {
				h.NoteActivity()
			}
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/health", handleHealth)

	r.Route("/api", func(r chi.Router) {
		r.Get("/status", s.handleStatus)
		r.Post("/render", handleRender)
		r.Post("/render/math", handleRenderMath)
		r.Post("/render/mermaid", handleRenderMermaid)
		r.Get("/model/info", s.handleModelInfo)

		r.Get("/config", s.handleConfigGet)
		r.Put("/config", s.handleConfigPut)
		r.Get("/config/schema", s.handleConfigSchema)

		r.Get("/conversation", s.handleConversationGet)
		r.Post("/conversation/messages", s.handleConversationPost)
		r.Post("/conversation/cancel", s.handleConversationCancel)
		r.Put("/conversation/messages/{id}", s.handleConversationMessagePut)
		r.Delete("/conversation/messages/{id}", s.handleConversationMessageDelete)
		r.Post("/conversation/messages/{id}/regenerate", s.handleConversationMessageRegenerate)

		r.Get("/sse", s.handleSSE)

		r.Get("/tools", s.handleToolsList)
		r.Get("/skills", s.handleSkillsList)
		r.Get("/providers", s.handleProvidersList)
		r.Post("/providers/{name}/models", s.handleProvidersModels)
		r.Post("/providers/{name}/test", s.handleProvidersTest)
		r.Post("/fallback_providers/{index}/models", s.handleFallbackProvidersModels)
		r.Post("/auxiliary/models", s.handleAuxiliaryModels)
		r.Post("/auxiliary/test", s.handleAuxiliaryTest)

		r.Get("/memory/stats", s.handleMemoryStats)
		r.Get("/memory/health", s.handleMemoryHealth)
		r.Get("/memory/report", s.handleMemoryReport)
		r.Get("/memory/{id}", s.handleMemoryGet)

		r.Get("/skills/stats", s.handleSkillsStats)

		r.Post("/upload", s.handleUpload)

		r.Post("/feedback", s.handleFeedback)
		r.Get("/suggestions", s.handleSuggestions)
		r.Post("/tts", s.handleTTS)

		// Browser extension endpoints
		r.Get("/browser-extension/check", s.extensionAuth(s.handleBrowserExtensionCheck))
		r.Post("/browser-extension/scrape", s.extensionAuth(s.handleBrowserExtensionScrape))
		r.Get("/browser-extension/poll", s.extensionAuth(s.handleBrowserExtensionPoll))
		r.Post("/browser-extension/result", s.extensionAuth(s.handleBrowserExtensionResult))
	})

	if s.opts.Config.Proxy.Enabled {
		r.Post("/v1/messages", s.handleV1Messages)
	}

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
	deps := s.currentDeps()
	if deps.Provider == nil {
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
		deps.Provider, deps.AuxProvider, deps.Storage,
		s.activeToolReg(), deps.AgentCfg, deps.Platform,
	)
	if deps.MemProvider != nil {
		eng.Memory().AddProvider(deps.MemProvider)
		if r, ok := deps.MemProvider.(memprovider.Recaller); ok {
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
			if mc.JudgeEnabled && deps.AuxProvider != nil {
				eng.SetConversationJudge(agent.NewLLMJudge(deps.AuxProvider))
			}
		}
	}
	wireEngineToHub(eng, s.streams)

	// Wire skills evolver and retriever
	if deps.SkillsEvolver != nil {
		eng.SetSkillsEvolver(deps.SkillsEvolver)
	}
	if deps.SkillsRetriever != nil {
		injectCount := s.opts.Config.Skills.InjectCount
		if injectCount <= 0 {
			injectCount = 3
		}
		ret := deps.SkillsRetriever
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
	return result.Response.Text(), nil
}

// StartGateway starts the gateway pump in the background if any platforms
// are configured and enabled. The pump runs until ctx is cancelled.
func (s *Server) StartGateway(ctx context.Context) {
	pump, err := gateway.NewPump(s.opts.Config.Gateway, s)
	if err != nil {
		mlog.Error("gateway: startup failed", mlog.String("err", err.Error()))
		return
	}
	if !pump.HasPlatforms() {
		mlog.Info("gateway: no enabled platforms, not starting")
		return
	}
	go pump.Start(ctx)
	mlog.Info("gateway: pump started")
}

// snippetsToActiveSkills converts a list of skill snippets to ActiveSkill structs.
func snippetsToActiveSkills(snippets []string) []agent.ActiveSkill {
	out := make([]agent.ActiveSkill, 0, len(snippets))
	for _, s := range snippets {
		out = append(out, agent.ActiveSkill{Body: s})
	}
	return out
}
