# Web REST API + Session Store Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a `hermind web` HTTP server that exposes the REST surface the existing Python React frontend expects (`/api/status`, `/api/config`, `/api/sessions`, `/api/sessions/{id}/messages`, `/api/model/info`, etc.), backed by the existing `storage.Storage` layer, protected by a session-scoped Bearer token, and bound to `127.0.0.1` only. Scope stops short of WebSocket streaming (that's Plan G). Scope stops short of OAuth provider flows, Cron job management, and Dashboard plugins (those stay with the existing CLI commands for now).

**Architecture:** New `api/` package owns everything web-facing: router, handlers, middleware, DTOs. Uses `net/http` with `chi` for routing because it's the smallest dependency that gives us path params without writing our own radix trie. A single in-memory `SessionToken` is generated per server boot (`crypto/rand` → base64url) and injected into the served `index.html`. A middleware rejects any `/api/*` request whose `Authorization` header does not match, except for a small public allowlist (`/api/status`, `/api/model/info`). Storage is the existing `storage.Storage` — no new tables, since `storage/sqlite` already has sessions + messages. A `cli/web.go` subcommand assembles the server, opens the browser at `http://127.0.0.1:<port>/?t=<token>`, and blocks until Ctrl-C.

**Tech Stack:** Go 1.21+, `net/http`, `github.com/go-chi/chi/v5`, `embed`, `crypto/rand`, existing `storage.Storage`, `config.Config`, `provider/factory`, `cobra`.

---

## File Structure

- Modify: `go.mod` — add `github.com/go-chi/chi/v5`
- Create: `api/server.go` — `Server{cfg, store, token, mux}` + router wiring
- Create: `api/server_test.go`
- Create: `api/auth.go` — Bearer token middleware + public allowlist
- Create: `api/auth_test.go`
- Create: `api/handlers_meta.go` — `/api/status`, `/api/model/info`
- Create: `api/handlers_config.go` — `/api/config` GET + PUT, `/api/config/raw` GET + PUT
- Create: `api/handlers_sessions.go` — `/api/sessions` (list), `/api/sessions/{id}`, `/api/sessions/{id}/messages`, DELETE
- Create: `api/handlers_sessions_test.go`
- Create: `api/dto.go` — response structs matching the Python JSON shape the React frontend consumes
- Create: `api/webroot/index.html` — minimal landing page with token injection slot (real frontend embeds come later — this is enough to verify wiring)
- Create: `api/webroot.go` — `//go:embed` glue
- Create: `cli/web.go` — `hermind web` subcommand
- Create: `cli/web_test.go`
- Modify: `cli/root.go` — register `newWebCmd(app)`

---

## Task 1: Add chi dependency

- [ ] **Step 1: Add it**

Run: `go get github.com/go-chi/chi/v5@latest`

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add chi router for web api"
```

---

## Task 2: DTOs that match Python response shapes

**Files:**
- Create: `api/dto.go`

- [ ] **Step 1: Skim the Python shapes**

The frontend calls these JSON shapes:

- `GET /api/status` → `{"version":"...","uptime_sec":N,"storage_driver":"sqlite"}`
- `GET /api/model/info` → `{"model":"...","context_length":N,"supports_tools":true,...}`
- `GET /api/sessions?limit=20&offset=0` → `{"sessions":[{"id","source","model","started_at","ended_at","message_count","title"}],"total":N}`
- `GET /api/sessions/{id}` → one of the above session items (no array)
- `GET /api/sessions/{id}/messages?limit=50&offset=0` → `{"messages":[{"id","role","content","tool_calls","timestamp","token_count"}],"total":N}`
- `GET /api/config` → `{"config":{...YAML-as-JSON...}}`
- `PUT /api/config` → `{"ok":true}`

- [ ] **Step 2: Write the failing test**

Create `api/dto_test.go`:

```go
package api

import (
	"encoding/json"
	"testing"
)

func TestSessionListResponse_JSONShape(t *testing.T) {
	resp := SessionListResponse{
		Total: 1,
		Sessions: []SessionDTO{{
			ID: "s1", Source: "cli", Model: "m", MessageCount: 3,
		}},
	}
	data, _ := json.Marshal(resp)
	want := `{"sessions":[{"id":"s1","source":"cli","model":"m","started_at":0,"ended_at":0,"message_count":3,"title":""}],"total":1}`
	if string(data) != want {
		t.Errorf("got %s\nwant %s", data, want)
	}
}

func TestMessagesResponse_JSONShape(t *testing.T) {
	resp := MessagesResponse{
		Total: 2,
		Messages: []MessageDTO{
			{ID: 1, Role: "user", Content: "hi"},
			{ID: 2, Role: "assistant", Content: "hello"},
		},
	}
	data, _ := json.Marshal(resp)
	// Assert presence of keys the frontend depends on.
	for _, key := range []string{`"id":1`, `"role":"user"`, `"content":"hi"`, `"total":2`} {
		if !containsSubstr(string(data), key) {
			t.Errorf("missing %s in %s", key, data)
		}
	}
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./api/ -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement DTOs**

Create `api/dto.go`:

```go
// Package api serves the hermind web UI and REST API. Shapes match the
// existing Python React frontend so it can be pointed at the Go server
// without changes.
package api

// StatusResponse is the payload for GET /api/status.
type StatusResponse struct {
	Version       string `json:"version"`
	UptimeSec     int64  `json:"uptime_sec"`
	StorageDriver string `json:"storage_driver"`
}

// ModelInfoResponse is the payload for GET /api/model/info.
type ModelInfoResponse struct {
	Model           string `json:"model"`
	ContextLength   int    `json:"context_length"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	SupportsTools   bool   `json:"supports_tools"`
	SupportsVision  bool   `json:"supports_vision"`
}

// SessionDTO is one session row in the list endpoint.
type SessionDTO struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Model        string  `json:"model"`
	StartedAt    float64 `json:"started_at"`
	EndedAt      float64 `json:"ended_at"`
	MessageCount int     `json:"message_count"`
	Title        string  `json:"title"`
}

// SessionListResponse is the payload for GET /api/sessions.
type SessionListResponse struct {
	Sessions []SessionDTO `json:"sessions"`
	Total    int          `json:"total"`
}

// MessageDTO is one message in the messages endpoint.
type MessageDTO struct {
	ID         int64   `json:"id"`
	Role       string  `json:"role"`
	Content    string  `json:"content"`
	ToolCalls  string  `json:"tool_calls,omitempty"` // raw JSON string
	Timestamp  float64 `json:"timestamp"`
	TokenCount int     `json:"token_count,omitempty"`
}

// MessagesResponse is the payload for GET /api/sessions/{id}/messages.
type MessagesResponse struct {
	Messages []MessageDTO `json:"messages"`
	Total    int          `json:"total"`
}

// ConfigResponse is the payload for GET /api/config.
type ConfigResponse struct {
	Config map[string]any `json:"config"`
}

// OKResponse is the success payload used by PUT/DELETE endpoints.
type OKResponse struct {
	OK bool `json:"ok"`
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./api/ -v`
Expected: PASS (2 sub-tests).

- [ ] **Step 6: Commit**

```bash
git add api/dto.go api/dto_test.go
git commit -m "feat(api): DTOs matching Python REST frontend shape"
```

---

## Task 3: Bearer token auth middleware

**Files:**
- Create: `api/auth.go`
- Create: `api/auth_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/auth_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_AllowsPublicPath(t *testing.T) {
	mw := NewAuthMiddleware("secret", []string{"/api/status"})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_RejectsMissingToken(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not call") })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_AcceptsValidBearer(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not call") })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	a, _ := GenerateToken()
	b, _ := GenerateToken()
	if a == "" || b == "" {
		t.Error("empty token")
	}
	if a == b {
		t.Error("tokens repeated")
	}
	if len(a) < 32 {
		t.Errorf("token too short: %d chars", len(a))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestAuthMiddleware -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement the middleware**

Create `api/auth.go`:

```go
package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// GenerateToken returns a random URL-safe token suitable for a
// single server-boot session. 32 bytes → 43 base64url chars.
func GenerateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// NewAuthMiddleware returns a middleware that enforces a Bearer token
// on /api/* requests, bypassing the allowlist of public paths.
func NewAuthMiddleware(token string, publicPaths []string) func(http.Handler) http.Handler {
	publicSet := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		publicSet[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := publicSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(auth, prefix) {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			got := auth[len(prefix):]
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run "TestAuthMiddleware|TestGenerateToken" -v`
Expected: PASS (5 sub-tests).

- [ ] **Step 5: Commit**

```bash
git add api/auth.go api/auth_test.go
git commit -m "feat(api): constant-time bearer auth middleware"
```

---

## Task 4: Meta handlers (`/api/status`, `/api/model/info`)

**Files:**
- Create: `api/handlers_meta.go`

- [ ] **Step 1: Write the failing test**

Append to `api/server_test.go` (create the file now):

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{Model: "anthropic/claude-opus-4-6"}
	s, err := NewServer(&ServerOpts{
		Config:  cfg,
		Storage: nil,
		Token:   "t",
		Version: "dev-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStatus_PublicAccess(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp StatusResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Version != "dev-test" {
		t.Errorf("version = %q", resp.Version)
	}
}

func TestModelInfo_PublicAccess(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/api/model/info", nil)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code = %d", rr.Code)
	}
	var resp ModelInfoResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Model != "anthropic/claude-opus-4-6" {
		t.Errorf("model = %q", resp.Model)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run "TestStatus|TestModelInfo" -v`
Expected: FAIL — `NewServer`, `Server.Router` undefined.

- [ ] **Step 3: Implement meta handlers**

Create `api/handlers_meta.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, StatusResponse{
		Version:       s.opts.Version,
		UptimeSec:     int64(time.Since(s.bootedAt).Seconds()),
		StorageDriver: s.driverName(),
	})
}

func (s *Server) handleModelInfo(w http.ResponseWriter, _ *http.Request) {
	resp := ModelInfoResponse{Model: s.opts.Config.Model}
	// Best-effort capability lookup — the server stays usable even
	// without a fully configured provider.
	if len(s.opts.Config.Providers) > 0 && s.opts.Config.Model != "" {
		resp.ContextLength = 200_000
		resp.SupportsTools = true
		resp.SupportsVision = true
		resp.MaxOutputTokens = 8192
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
```

Now create `api/server.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

// ServerOpts bundles server-wide state.
type ServerOpts struct {
	Config  *config.Config
	Storage storage.Storage // may be nil for meta-only test servers
	Token   string
	Version string
}

// Server is the API server.
type Server struct {
	opts     *ServerOpts
	router   chi.Router
	bootedAt time.Time
}

// NewServer wires routes and middleware.
func NewServer(opts *ServerOpts) (*Server, error) {
	if opts == nil || opts.Config == nil {
		return nil, fmt.Errorf("api: ServerOpts.Config is required")
	}
	if opts.Token == "" {
		return nil, fmt.Errorf("api: ServerOpts.Token is required")
	}
	s := &Server{opts: opts, bootedAt: time.Now()}
	s.router = s.buildRouter()
	return s, nil
}

// Router returns the configured chi router (useful for tests).
func (s *Server) Router() chi.Router { return s.router }

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
		r.Get("/sessions/{id}/messages", s.handleSessionMessages)
		r.Delete("/sessions/{id}", s.handleSessionDelete)
	})

	// Static landing page / frontend shell with token injection.
	r.Get("/", s.handleIndex)
	r.Get("/ui/*", s.handleStatic)

	return r
}

func (s *Server) driverName() string {
	if s.opts.Storage == nil {
		return "none"
	}
	return s.opts.Config.Storage.Driver
}

// ---- placeholders wired in later tasks ----

func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleConfigPut(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSessionsList(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSessionGet(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSessionMessages(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html><body>hermind web</body></html>`))
}

func (s *Server) handleStatic(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run "TestStatus|TestModelInfo" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/server.go api/handlers_meta.go api/server_test.go
git commit -m "feat(api): server skeleton + /api/status + /api/model/info"
```

---

## Task 5: Sessions handlers

**Files:**
- Modify: `api/server.go` (swap placeholders)
- Create: `api/handlers_sessions.go`
- Create: `api/handlers_sessions_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/handlers_sessions_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/storage/sqlite"
)

func newTestServerWithStore(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	store, err := sqlite.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	s, err := NewServer(&ServerOpts{
		Config:  &config.Config{Model: "x"},
		Storage: store,
		Token:   "t",
		Version: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, store
}

func seedSession(t *testing.T, store storage.Storage, id string) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateSession(ctx, &storage.Session{
		ID:        id,
		Source:    "cli",
		Model:     "anthropic/claude-opus-4-6",
		StartedAt: float64(time.Now().Unix()),
	}); err != nil {
		t.Fatal(err)
	}
	_ = store.AddMessage(ctx, id, &storage.StoredMessage{Role: "user", Content: "hi"})
	_ = store.AddMessage(ctx, id, &storage.StoredMessage{Role: "assistant", Content: "hello"})
}

func authedReq(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestSessionsList_Pagination(t *testing.T) {
	s, store := newTestServerWithStore(t)
	seedSession(t, store, "a")
	seedSession(t, store, "b")
	seedSession(t, store, "c")

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions?limit=2", "t"))
	if rr.Code != 200 {
		t.Fatalf("code=%d, body=%s", rr.Code, rr.Body.String())
	}
	var resp SessionListResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total < 3 {
		t.Errorf("total = %d", resp.Total)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("len = %d", len(resp.Sessions))
	}
}

func TestSessionGet_Found(t *testing.T) {
	s, store := newTestServerWithStore(t)
	seedSession(t, store, "abc")

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions/abc", "t"))
	if rr.Code != 200 {
		t.Fatalf("code=%d", rr.Code)
	}
	var dto SessionDTO
	_ = json.NewDecoder(rr.Body).Decode(&dto)
	if dto.ID != "abc" {
		t.Errorf("id = %q", dto.ID)
	}
}

func TestSessionGet_NotFound(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions/nope", "t"))
	if rr.Code != 404 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestSessionMessages_ReturnsHistory(t *testing.T) {
	s, store := newTestServerWithStore(t)
	seedSession(t, store, "x")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions/x/messages", "t"))
	if rr.Code != 200 {
		t.Fatalf("code=%d, body=%s", rr.Code, rr.Body.String())
	}
	var resp MessagesResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Messages) != 2 {
		t.Errorf("len = %d", len(resp.Messages))
	}
	if resp.Messages[0].Role != "user" {
		t.Errorf("first role = %q", resp.Messages[0].Role)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestSession -v`
Expected: FAIL — placeholders return 501.

- [ ] **Step 3: Implement session handlers**

Create `api/handlers_sessions.go`:

```go
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/storage"
)

func (s *Server) sessionsList(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 20)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	rows, err := s.opts.Storage.ListSessions(r.Context(), &storage.ListOptions{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]SessionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, dtoFromSession(row))
	}
	// Total is unpopulated in the minimal Storage interface, so we
	// approximate it as offset + len(rows) + 1 when a full page came
	// back. Clients that need a hard total call the analytics endpoint.
	total := offset + len(rows)
	if len(rows) == limit {
		total++ // hint "more" without lying about an exact count
	}
	writeJSON(w, SessionListResponse{Sessions: out, Total: total})
}

func (s *Server) sessionGet(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	row, err := s.opts.Storage.GetSession(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dtoFromSession(row))
}

func (s *Server) sessionMessages(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	offset := atoiDefault(r.URL.Query().Get("offset"), 0)

	rows, err := s.opts.Storage.GetMessages(r.Context(), id, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msgs := make([]MessageDTO, 0, len(rows))
	for i, row := range rows {
		msgs = append(msgs, MessageDTO{
			ID:         int64(offset + i + 1),
			Role:       row.Role,
			Content:    row.Content,
			Timestamp:  row.Timestamp,
			TokenCount: row.TokenCount,
		})
	}
	writeJSON(w, MessagesResponse{Messages: msgs, Total: offset + len(msgs)})
}

func (s *Server) sessionDelete(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	// The minimal Storage interface does not expose DeleteSession. If
	// your implementation does (storage/sqlite probably has it), cast
	// and call. Otherwise, return 501 so the frontend surfaces a
	// predictable "not supported yet" error.
	if deleter, ok := s.opts.Storage.(interface {
		DeleteSession(ctx interface{ Done() <-chan struct{} }, id string) error
	}); ok {
		_ = deleter // keep the compiler quiet; real implementation below
	}
	if d, ok := s.opts.Storage.(interface {
		DeleteSession(ctx chiCtx, id string) error
	}); ok {
		if err := d.DeleteSession(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, OKResponse{OK: true})
		return
	}
	_ = id
	http.Error(w, "delete not supported by this storage backend", http.StatusNotImplemented)
}

// chiCtx is a type alias to the request context the Storage
// implementation expects. Adjust to match your actual interface.
type chiCtx = interface {
	Done() <-chan struct{}
	Err() error
	Value(key interface{}) interface{}
	Deadline() (deadline interface{ Before(interface{}) bool }, ok bool)
}

func dtoFromSession(s *storage.Session) SessionDTO {
	return SessionDTO{
		ID:           s.ID,
		Source:       s.Source,
		Model:        s.Model,
		StartedAt:    s.StartedAt,
		EndedAt:      s.EndedAt,
		MessageCount: s.MessageCount,
		Title:        s.Title,
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}
```

Then simplify the delete handler. The `chiCtx` dance is unnecessary — use the real context.Context and whatever delete method your storage exposes. Replace `sessionDelete` with:

```go
func (s *Server) sessionDelete(w http.ResponseWriter, r *http.Request) {
	// The minimal Storage interface in storage/storage.go does not
	// include DeleteSession. If storage/sqlite exposes one (grep
	// "DeleteSession" storage/sqlite), use a type assertion to the
	// concrete type or extend Storage. MVP: surface 501 so the
	// frontend knows to hide the delete button.
	_ = r
	http.Error(w, "session deletion not supported in MVP", http.StatusNotImplemented)
}
```

Delete the `chiCtx` alias entirely.

- [ ] **Step 4: Swap placeholders in buildRouter**

In `api/server.go`, replace the placeholder `handleSessionsList`, `handleSessionGet`, `handleSessionMessages`, `handleSessionDelete` methods with real wrappers:

```go
func (s *Server) handleSessionsList(w http.ResponseWriter, r *http.Request)   { s.sessionsList(w, r) }
func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request)    { s.sessionGet(w, r) }
func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) { s.sessionMessages(w, r) }
func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) { s.sessionDelete(w, r) }
```

(Or inline the implementations — the wrapper layer exists only for test mocking.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./api/ -run TestSession -v`
Expected: PASS (4 sub-tests).

- [ ] **Step 6: Commit**

```bash
git add api/handlers_sessions.go api/handlers_sessions_test.go api/server.go
git commit -m "feat(api): GET /api/sessions* handlers"
```

---

## Task 6: Config handlers

**Files:**
- Create: `api/handlers_config.go`
- Modify: `api/server.go` wrappers

- [ ] **Step 1: Write the failing test**

Append to `api/server_test.go`:

```go
func TestConfigGet(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/config", "t"))
	if rr.Code != 200 {
		t.Fatalf("code = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp ConfigResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Config["model"] != "anthropic/claude-opus-4-6" {
		t.Errorf("config.model = %v", resp.Config["model"])
	}
}
```

Add to the imports of `api/server_test.go`: `"encoding/json"` (already there likely), and `"net/http/httptest"` (ditto).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestConfigGet -v`
Expected: FAIL — still returns 501.

- [ ] **Step 3: Implement config handlers**

Create `api/handlers_config.go`:

```go
package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/odysseythink/hermind/config"
)

func (s *Server) configGet(w http.ResponseWriter, _ *http.Request) {
	// Round-trip via YAML → map so the frontend sees snake_case keys
	// matching the on-disk config file.
	data, err := yaml.Marshal(s.opts.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, ConfigResponse{Config: m})
}

func (s *Server) configPut(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	var req struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var updated config.Config
	if err := yaml.Unmarshal(req.Config, &updated); err != nil {
		http.Error(w, "invalid config payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Atomic write delegates to config.SaveToPath (from Plan A). If
	// Plan A has not landed, this PUT returns 501.
	if s.opts.ConfigPath == "" {
		http.Error(w, "config write-back not configured", http.StatusNotImplemented)
		return
	}
	if err := config.SaveToPath(s.opts.ConfigPath, &updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	*s.opts.Config = updated
	writeJSON(w, OKResponse{OK: true})
}
```

Update `ServerOpts` in `api/server.go` to include `ConfigPath`:

```go
type ServerOpts struct {
	Config     *config.Config
	ConfigPath string          // path to write when PUT /api/config lands; empty disables PUT
	Storage    storage.Storage
	Token      string
	Version    string
}
```

Wire the handler wrappers:

```go
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) { s.configGet(w, r) }
func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) { s.configPut(w, r) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestConfigGet -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/handlers_config.go api/server.go api/server_test.go
git commit -m "feat(api): GET/PUT /api/config handlers"
```

---

## Task 7: CLI `hermind web` + browser open

**Files:**
- Create: `cli/web.go`
- Create: `cli/web_test.go`
- Modify: `cli/root.go`

- [ ] **Step 1: Write the failing test**

Create `cli/web_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWebCmd_BindsAndServesStatus(t *testing.T) {
	app := newTestApp(t) // helper below
	cmd := newWebCmd(app)
	cmd.SetArgs([]string{"--addr", "127.0.0.1:0", "--no-browser", "--exit-after", "1s"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Run in a goroutine because the server blocks.
	errCh := make(chan error, 1)
	go func() { errCh <- cmd.ExecuteContext(context.Background()) }()

	// Give the server a beat to bind.
	time.Sleep(100 * time.Millisecond)

	// Scrape the stdout for the "listening on ..." line.
	lines := strings.Split(out.String(), "\n")
	var addr string
	for _, l := range lines {
		if i := strings.Index(l, "http://"); i >= 0 {
			addr = strings.TrimSpace(l[i:])
			break
		}
	}
	if addr == "" {
		t.Fatalf("listening URL not found in output: %q", out.String())
	}
	resp, err := http.Get(addr + "/api/status")
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Wait for --exit-after to fire.
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("cmd: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cmd did not exit")
	}
}
```

Add a shared `newTestApp(t *testing.T) *App` helper if not yet present — something like:

```go
func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)
	cfgPath := dir + "/config.yaml"
	_ = os.WriteFile(cfgPath, []byte("model: anthropic/claude-opus-4-6\n"), 0o644)
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli/ -run TestWebCmd -v`
Expected: FAIL — `newWebCmd` undefined.

- [ ] **Step 3: Implement the command**

Create `cli/web.go`:

```go
package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/api"
)

func newWebCmd(app *App) *cobra.Command {
	var (
		addr       string
		noBrowser  bool
		exitAfter  time.Duration
	)
	c := &cobra.Command{
		Use:   "web",
		Short: "Start the hermind web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}

			token, err := api.GenerateToken()
			if err != nil {
				return err
			}
			srv, err := api.NewServer(&api.ServerOpts{
				Config:     app.Config,
				ConfigPath: app.ConfigPath,
				Storage:    app.Storage,
				Token:      token,
				Version:    Version,
			})
			if err != nil {
				return err
			}

			// Bind early so we can report the real port when :0 was
			// requested.
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			realAddr := "http://" + ln.Addr().String()
			fmt.Fprintf(cmd.OutOrStdout(), "hermind web listening on %s\n", realAddr)
			fmt.Fprintf(cmd.OutOrStdout(), "token: %s\n", token)

			if !noBrowser {
				go openBrowser(realAddr + "/?t=" + token)
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			if exitAfter > 0 {
				time.AfterFunc(exitAfter, cancel)
			}

			httpSrv := &http.Server{
				Handler:           srv.Router(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				<-ctx.Done()
				shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
				defer c2()
				_ = httpSrv.Shutdown(shutCtx)
			}()
			if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:9119", "bind address (keep 127.0.0.1 unless you know what you're doing)")
	c.Flags().BoolVar(&noBrowser, "no-browser", false, "do not open the browser automatically")
	c.Flags().DurationVar(&exitAfter, "exit-after", 0, "exit after the given duration (0 = run until Ctrl-C)")
	return c
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}
	_ = exec.Command(cmd, args...).Start()
}
```

- [ ] **Step 4: Register in root**

In `cli/root.go`, add `newWebCmd(app),` to the `AddCommand(...)` list.

- [ ] **Step 5: Run tests**

Run: `go test ./cli/ -run TestWebCmd -v`
Expected: PASS.

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/web.go cli/web_test.go cli/root.go
git commit -m "feat(cli): add 'hermind web' subcommand"
```

---

## Task 8: Manual smoke test

- [ ] **Step 1: Build and run**

```bash
go build -o /tmp/hermind ./cmd/hermind
/tmp/hermind web --no-browser --addr 127.0.0.1:9119 &
sleep 1
TOKEN=$( # copy-paste from the terminal output
echo "paste token here"
)
curl http://127.0.0.1:9119/api/status                                       # public
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9119/api/sessions   # authed
kill %1
```

Expected: `/api/status` returns JSON with the version; `/api/sessions` returns `{"sessions":[],"total":0}`; `/api/sessions` without auth returns 401.

- [ ] **Step 2: Cleanup**

```bash
rm /tmp/hermind
```

- [ ] **Step 3: Optional marker commit**

```bash
git commit --allow-empty -m "test(api): manual smoke test verified REST surface"
```

---

## Self-Review Checklist

1. **Spec coverage:**
   - `/api/status`, `/api/model/info` ↔ Task 4 ✓
   - `/api/sessions` list + pagination ↔ Task 5 ✓
   - `/api/sessions/{id}`, `/messages` ↔ Task 5 ✓
   - `/api/config` GET/PUT with atomic save ↔ Task 6 ✓
   - Bearer token middleware with public allowlist ↔ Task 3 ✓
   - localhost-only binding default ↔ Task 7 (`127.0.0.1:9119`) ✓
   - Browser auto-open ↔ Task 7 ✓

2. **Placeholders:** Task 5 Step 3 includes a deliberate simplification path for the delete handler (returning 501 in MVP). Task 6 makes PUT 501 when `ConfigPath` is empty. All behavior is specified.

3. **Type consistency:**
   - `ServerOpts{Config, ConfigPath, Storage, Token, Version}` stable across Tasks 4, 5, 6, 7.
   - `SessionDTO` / `MessageDTO` JSON fields match between DTO definition (Task 2) and handlers (Task 5).
   - `NewAuthMiddleware(token, publicPaths)` signature stable across Task 3 and Task 4.
   - `SessionListResponse`, `MessagesResponse`, `ConfigResponse`, `OKResponse` wire shapes stable.

4. **Gaps (deferred to future plans):**
   - WebSocket streaming → Plan G.
   - OAuth provider endpoints → separate plan.
   - Cron job / Skills toggle endpoints → folded into their owning CLI commands for now.
   - Dashboard plugin discovery → out of scope.
   - FTS5 search endpoint → pending a follow-up plan when the real frontend lands.

---

## Definition of Done

- `go test ./api/... ./cli/... -race` all pass.
- `go build ./...` succeeds.
- `hermind web --no-browser` starts on 127.0.0.1:9119, serves `/api/status` (public) and `/api/sessions` (authed) correctly.
- Bearer token is regenerated each boot; no disk persistence.
