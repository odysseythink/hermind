package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func (m *mockStorage) seedSessionFull(id, source, model, systemPrompt, title string) {
	m.sessions = append([]*storage.Session{{
		ID:           id,
		Source:       source,
		Model:        model,
		SystemPrompt: systemPrompt,
		StartedAt:    time.Now().UTC(),
		MessageCount: 0,
		Title:        title,
	}}, m.sessions...)
	m.messages[id] = []*storage.StoredMessage{}
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
	for _, s := range m.sessions {
		if s.ID == id {
			if u.Title != "" {
				s.Title = u.Title
			}
			if u.EndedAt != nil {
				s.EndedAt = u.EndedAt
			}
			if u.EndReason != "" {
				s.EndReason = u.EndReason
			}
			if u.MessageCount != nil {
				s.MessageCount = *u.MessageCount
			}
			if u.SystemPrompt != nil {
				s.SystemPrompt = *u.SystemPrompt
			}
			if u.Model != nil {
				s.Model = *u.Model
			}
			return nil
		}
	}
	return storage.ErrNotFound
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

func TestPatchSession_RenamesTitle(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("sess-rename")

	rr := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"new title"}`)
	req := httptest.NewRequest("PATCH", "/api/sessions/sess-rename", body)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	if err := json.NewDecoder(rr.Body).Decode(&dto); err != nil {
		t.Fatal(err)
	}
	if dto.Title != "new title" {
		t.Errorf("title = %q, want %q", dto.Title, "new title")
	}
	if dto.ID != "sess-rename" {
		t.Errorf("id = %q, want %q", dto.ID, "sess-rename")
	}
}

func TestPatchSession_EmptyTitle_Returns400(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s1")

	for _, body := range []string{`{"title":""}`, `{"title":"   "}`} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/api/sessions/s1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer t")
		req.Header.Set("Content-Type", "application/json")
		s.Router().ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("body=%q: code=%d, want 400", body, rr.Code)
		}
	}
}

func TestPatchSession_TooLong_Returns400(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s2")
	// Use MaxSessionTitleBytes+1 bytes so the new byte-length limit triggers.
	body := `{"title":"` + strings.Repeat("x", MaxSessionTitleBytes+1) + `"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s2", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_NotFound_Returns404(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/ghost",
		strings.NewReader(`{"title":"anything"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("code=%d, want 404", rr.Code)
	}
}

func TestPatchSession_MissingToken_Returns401(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSession("s3")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s3",
		strings.NewReader(`{"title":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("code=%d, want 401", rr.Code)
	}
}

func TestGetSession_ReturnsSystemPromptField(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-dto-1", "web", "claude-opus-4-7", "You are a helper.", "Title 1")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sessions/s-dto-1", nil)
	req.Header.Set("Authorization", "Bearer t")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	if got, want := dto.SystemPrompt, "You are a helper."; got != want {
		t.Errorf("SystemPrompt = %q, want %q", got, want)
	}
}

func TestPatchSession_UpdatesModelAndSystemPrompt(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p1", "web", "claude-opus-4-7", "orig", "t")

	body := `{"model":"claude-sonnet-4-6","system_prompt":"new prompt"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p1", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "claude-sonnet-4-6", dto.Model)
	assert.Equal(t, "new prompt", dto.SystemPrompt)
	assert.Equal(t, "t", dto.Title) // unchanged
}

func TestPatchSession_OnlyTitle_StillWorks(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p2", "web", "claude-opus-4-7", "orig", "old")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p2",
		strings.NewReader(`{"title":"new"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "new", dto.Title)
	assert.Equal(t, "orig", dto.SystemPrompt)
}

func TestPatchSession_EnforcesSystemPromptSizeLimit(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p3", "web", "claude-opus-4-7", "orig", "t")

	tooBig := strings.Repeat("a", MaxSystemPromptBytes+1)
	body := fmt.Sprintf(`{"system_prompt":%q}`, tooBig)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p3", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_EnforcesModelNameLimit(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p4", "web", "claude-opus-4-7", "orig", "t")

	body := fmt.Sprintf(`{"model":%q}`, strings.Repeat("m", MaxModelNameBytes+1))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p4", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_AllowsEmptyStringToClear(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p5", "web", "claude-opus-4-7", "orig", "t")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p5",
		strings.NewReader(`{"system_prompt":""}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "", dto.SystemPrompt)
}
