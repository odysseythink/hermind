package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/storage"
)

// mockStorage is a minimal in-memory storage.Storage satisfying just
// enough of the interface for the API handlers under test.
type mockStorage struct {
	sessions []*storage.Session
	messages map[string][]*storage.StoredMessage
	// Overrides for error-injection tests.
	listErr error
	getErr  error
}

func newMockStorage() *mockStorage {
	return &mockStorage{messages: make(map[string][]*storage.StoredMessage)}
}

func (m *mockStorage) seedSession(id string) {
	m.sessions = append([]*storage.Session{{
		ID:           id,
		Source:       "cli",
		Model:        "anthropic/claude-opus-4-6",
		StartedAt:    time.Now().UTC(),
		MessageCount: 2,
		Title:        "t-" + id,
	}}, m.sessions...)
	m.messages[id] = []*storage.StoredMessage{
		{ID: 1, SessionID: id, Role: "user", Content: "hi", Timestamp: time.Now().UTC()},
		{ID: 2, SessionID: id, Role: "assistant", Content: "hello", Timestamp: time.Now().UTC()},
	}
}

func (m *mockStorage) CreateSession(ctx context.Context, s *storage.Session) error {
	m.sessions = append([]*storage.Session{s}, m.sessions...)
	return nil
}
func (m *mockStorage) GetSession(ctx context.Context, id string) (*storage.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, storage.ErrNotFound
}
func (m *mockStorage) UpdateSession(ctx context.Context, id string, u *storage.SessionUpdate) error {
	return nil
}
func (m *mockStorage) ListSessions(ctx context.Context, opts *storage.ListOptions) ([]*storage.Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	limit := len(m.sessions)
	if opts != nil && opts.Limit > 0 && opts.Limit < limit {
		limit = opts.Limit
	}
	out := make([]*storage.Session, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, m.sessions[i])
	}
	return out, nil
}
func (m *mockStorage) AddMessage(ctx context.Context, sid string, msg *storage.StoredMessage) error {
	m.messages[sid] = append(m.messages[sid], msg)
	return nil
}
func (m *mockStorage) GetMessages(ctx context.Context, sid string, limit, offset int) ([]*storage.StoredMessage, error) {
	all := m.messages[sid]
	if offset > len(all) {
		offset = len(all)
	}
	all = all[offset:]
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
func (m *mockStorage) SearchMessages(ctx context.Context, q string, opts *storage.SearchOptions) ([]*storage.SearchResult, error) {
	return nil, nil
}
func (m *mockStorage) UpdateSystemPrompt(ctx context.Context, sid, p string) error { return nil }
func (m *mockStorage) UpdateUsage(ctx context.Context, sid string, u *storage.UsageUpdate) error {
	return nil
}
func (m *mockStorage) SaveMemory(ctx context.Context, mem *storage.Memory) error { return nil }
func (m *mockStorage) GetMemory(ctx context.Context, id string) (*storage.Memory, error) {
	return nil, storage.ErrNotFound
}
func (m *mockStorage) SearchMemories(ctx context.Context, q string, o *storage.MemorySearchOptions) ([]*storage.Memory, error) {
	return nil, nil
}
func (m *mockStorage) DeleteMemory(ctx context.Context, id string) error { return nil }
func (m *mockStorage) WithTx(ctx context.Context, fn func(storage.Tx) error) error {
	return nil
}
func (m *mockStorage) Close() error   { return nil }
func (m *mockStorage) Migrate() error { return nil }

func newTestServerWithStore(t *testing.T) (*Server, *mockStorage) {
	t.Helper()
	store := newMockStorage()
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

func TestSessionsList_Pagination(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("a")
	store.seedSession("b")
	store.seedSession("c")

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions?limit=2", "t"))
	if rr.Code != 200 {
		t.Fatalf("code=%d, body=%s", rr.Code, rr.Body.String())
	}
	var resp SessionListResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Sessions) != 2 {
		t.Errorf("len = %d (want 2)", len(resp.Sessions))
	}
}

func TestSessionsList_Offset(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("a")
	store.seedSession("b")
	store.seedSession("c")

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("GET", "/api/sessions?limit=2&offset=2", "t"))
	if rr.Code != 200 {
		t.Fatalf("code=%d", rr.Code)
	}
	var resp SessionListResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Sessions) != 1 {
		t.Errorf("len = %d (want 1)", len(resp.Sessions))
	}
}

func TestSessionGet_Found(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("abc")

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
	if dto.StartedAt == 0 {
		t.Errorf("started_at = 0")
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
	store.seedSession("x")
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

func TestSessionDelete_NotImplemented(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("x")
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, authedReq("DELETE", "/api/sessions/x", "t"))
	if rr.Code != 501 {
		t.Errorf("code = %d, want 501", rr.Code)
	}
}

func TestSessionStatus_ReportsDriver(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	s.opts.Config.Storage.Driver = "sqlite"
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/status", nil))
	var resp StatusResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.StorageDriver != "sqlite" {
		t.Errorf("driver = %q", resp.StorageDriver)
	}
}
