package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

//go:embed webroot/*
var webroot embed.FS

// GatewayController is the subset of cli/gatewayctl.Controller that
// the REST layer consumes. Keeping it as an interface avoids a cyclic
// import (cli depends on api, not the other way around) and lets
// handler tests stub it.
type GatewayController interface {
	// Apply performs a stop-rebuild-start cycle on the underlying
	// gateway subsystem. Returns ErrApplyInProgress if an Apply is
	// already running.
	Apply(ctx context.Context) (ApplyResult, error)

	// TestPlatform runs the platform's descriptor.Test for key.
	// Errors are surfaced verbatim; callers inspect the value for
	// user-facing mapping (ErrTestNotImplemented / ErrUnknownPlatformKey).
	TestPlatform(ctx context.Context, key string) error
}

// Sentinel errors shared between the API handlers and the controller
// implementation (cli/gatewayctl aliases these). Handlers use
// errors.Is against them regardless of which side returned the error.
var (
	ErrApplyInProgress    = errors.New("apply already in progress")
	ErrTestNotImplemented = errors.New("test not implemented for this platform type")
	ErrUnknownPlatformKey = errors.New("unknown platform key")
)

// ServerOpts bundles server-wide state.
type ServerOpts struct {
	// Config is the live config the server reflects. Required.
	Config *config.Config

	// ConfigPath is where PUT /api/config writes back to. When empty,
	// PUT returns 501.
	ConfigPath string

	// Storage is the backing store for sessions/messages. May be nil
	// for meta-only test servers; storage-backed endpoints return 503
	// in that case.
	Storage storage.Storage

	// Token is the Bearer token required on authed endpoints.
	Token string

	// Version stamps GET /api/status.
	Version string

	// Streams is the hook the WebSocket / SSE agent attaches to. Nil
	// means no streaming is available; the hub helper on the server
	// returns a no-op that accepts and drops events. Set to a real
	// StreamHub (e.g. NewMemoryStreamHub) when streaming is wanted.
	Streams StreamHub

	// Controller manages the gateway lifecycle. nil means the four
	// /api/platforms/* endpoints return 503 Service Unavailable.
	Controller GatewayController
}

// Server is the API server.
type Server struct {
	opts     *ServerOpts
	router   chi.Router
	bootedAt time.Time
	streams  StreamHub
}

// NewServer wires routes and middleware.
func NewServer(opts *ServerOpts) (*Server, error) {
	if opts == nil || opts.Config == nil {
		return nil, fmt.Errorf("api: ServerOpts.Config is required")
	}
	if opts.Token == "" {
		return nil, fmt.Errorf("api: ServerOpts.Token is required")
	}
	streams := opts.Streams
	if streams == nil {
		streams = NewMemoryStreamHub()
	}
	s := &Server{opts: opts, bootedAt: time.Now(), streams: streams}
	s.router = s.buildRouter()
	return s, nil
}

// Router returns the configured chi router (useful for tests and the
// parallel WebSocket agent that needs to mount additional routes).
func (s *Server) Router() chi.Router { return s.router }

// Streams exposes the StreamHub so the WebSocket/SSE agent can attach
// per-session subscribers without reaching into server internals.
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

	public := []string{"/api/status", "/api/model/info"}
	auth := NewAuthMiddleware(s.opts.Token, public)

	r.Route("/api", func(r chi.Router) {
		r.Use(auth)

		r.Get("/status", s.handleStatus)
		r.Get("/model/info", s.handleModelInfo)

		r.Get("/config", s.handleConfigGet)
		r.Put("/config", s.handleConfigPut)

		r.Get("/sessions", s.handleSessionsList)
		r.Get("/sessions/{id}", s.handleSessionGet)
		r.Delete("/sessions/{id}", s.handleSessionDelete)
		r.Get("/sessions/{id}/messages", s.handleSessionMessages)
		r.Get("/sessions/{id}/stream/ws", s.handleSessionStreamWS)
		r.Get("/sessions/{id}/stream/sse", s.handleSessionStreamSSE)

		r.Get("/tools", s.handleToolsList)
		r.Get("/skills", s.handleSkillsList)
		r.Get("/providers", s.handleProvidersList)
		r.Get("/platforms/schema", s.handlePlatformsSchema)
		r.Post("/platforms/{key}/reveal", s.handlePlatformReveal)
		r.Post("/platforms/{key}/test", s.handlePlatformTest)
		r.Post("/platforms/apply", s.handlePlatformsApply)
	})

	// Static landing page / frontend shell.
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

// handleIndex serves the embedded landing page with the server token
// substituted in so the bundled frontend can authenticate without the
// user pasting a token.
func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(webroot, "webroot/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rendered := strings.ReplaceAll(string(data), "{{TOKEN}}", s.opts.Token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(rendered))
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

// writeJSON is the single entry point for JSON responses; it centralizes
// the Content-Type and encoder configuration.
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
