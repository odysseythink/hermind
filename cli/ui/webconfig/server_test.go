package webconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("model: old\n"), 0o644)
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, p
}

func TestGetConfig(t *testing.T) {
	ts, _ := newServer(t)
	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "old" {
		t.Errorf("got %v", body["model"])
	}
}

func TestPostConfigAndSave(t *testing.T) {
	ts, p := newServer(t)
	payload, _ := json.Marshal(map[string]any{"path": "model", "value": "new"})
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if err != nil || resp.StatusCode >= 300 {
		t.Fatalf("post: %v %v", err, resp.Status)
	}
	resp2, err := http.Post(ts.URL+"/api/save", "application/json", nil)
	if err != nil || resp2.StatusCode >= 300 {
		t.Fatalf("save: %v %v", err, resp2.Status)
	}
	raw, _ := os.ReadFile(p)
	if !bytes.Contains(raw, []byte("model: new")) {
		t.Errorf("file not updated:\n%s", raw)
	}
}

func TestGetConfigMasksSecrets(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("providers:\n  anthropic:\n    api_key: supersecret\n"), 0o644)
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	if bytes.Contains(buf, []byte("supersecret")) {
		t.Errorf("secret leaked:\n%s", buf)
	}
}

func TestSchemaEndpoint(t *testing.T) {
	ts, _ := newServer(t)
	resp, err := http.Get(ts.URL + "/api/schema")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var fields []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) == 0 {
		t.Error("schema empty")
	}
}

func TestPostConfigRejectsUnknownPath(t *testing.T) {
	ts, _ := newServer(t)
	payload, _ := json.Marshal(map[string]any{"path": "not.a.field", "value": "x"})
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPostConfigRejectsListKind(t *testing.T) {
	ts, _ := newServer(t)
	payload, _ := json.Marshal(map[string]any{"path": "providers", "value": "x"})
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPostConfigAcceptsBoolAndInt(t *testing.T) {
	ts, p := newServer(t)
	// Bool: agent.compression.enabled
	payload, _ := json.Marshal(map[string]any{"path": "agent.compression.enabled", "value": true})
	resp, _ := http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if resp.StatusCode >= 300 {
		t.Errorf("bool post failed: %d", resp.StatusCode)
	}
	// Int: agent.max_turns
	payload, _ = json.Marshal(map[string]any{"path": "agent.max_turns", "value": 42})
	resp, _ = http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if resp.StatusCode >= 300 {
		t.Errorf("int post failed: %d", resp.StatusCode)
	}
	http.Post(ts.URL+"/api/save", "application/json", nil)
	raw, _ := os.ReadFile(p)
	// Both values should be written without string quotes in YAML.
	if !bytes.Contains(raw, []byte("enabled: true")) {
		t.Errorf("bool not serialized as YAML bool:\n%s", raw)
	}
	if !bytes.Contains(raw, []byte("max_turns: 42")) {
		t.Errorf("int not serialized as YAML int:\n%s", raw)
	}
}

func TestRevealRejectsCrossOrigin(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("memory:\n  honcho:\n    api_key: s\n"), 0o644)
	s, _ := New(p)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"path": "memory.honcho.api_key"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/reveal", bytes.NewReader(body))
	req.Header.Set("Origin", "http://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestProvidersGETMasksAPIKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("providers:\n  anthropic:\n    provider: anthropic\n    api_key: sk-secret\n    model: claude-opus-4-6\n"), 0o644)
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/providers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["key"] != "anthropic" {
		t.Fatalf("got %+v", list)
	}
	if list[0]["api_key"] != "••••" {
		t.Errorf("api_key not masked: %q", list[0]["api_key"])
	}
	if list[0]["model"] != "claude-opus-4-6" {
		t.Errorf("model lost: %q", list[0]["model"])
	}
}

func TestProvidersAddSetDelete(t *testing.T) {
	ts, _ := newServer(t)

	post := func(body map[string]any) *http.Response {
		b, _ := json.Marshal(body)
		resp, err := http.Post(ts.URL+"/api/providers", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	// Add
	if r := post(map[string]any{"op": "add", "key": "openai"}); r.StatusCode != http.StatusNoContent {
		t.Fatalf("add: got %d", r.StatusCode)
	}
	// Set api_key
	if r := post(map[string]any{"op": "set", "key": "openai", "field": "api_key", "value": "sk-123"}); r.StatusCode != http.StatusNoContent {
		t.Fatalf("set: got %d", r.StatusCode)
	}

	// GET shows masked key
	resp, err := http.Get(ts.URL + "/api/providers")
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0]["key"] != "openai" || list[0]["api_key"] != "••••" {
		t.Fatalf("unexpected state: %+v", list)
	}

	// Reveal returns the real key
	revBody, _ := json.Marshal(map[string]string{"path": "providers.openai.api_key"})
	resp, err = http.Post(ts.URL+"/api/reveal", "application/json", bytes.NewReader(revBody))
	if err != nil {
		t.Fatal(err)
	}
	var rev map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&rev)
	resp.Body.Close()
	if rev["value"] != "sk-123" {
		t.Errorf("reveal: got %q", rev["value"])
	}

	// Delete
	if r := post(map[string]any{"op": "delete", "key": "openai"}); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: got %d", r.StatusCode)
	}
	resp, err = http.Get(ts.URL + "/api/providers")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 0 {
		t.Errorf("expected empty, got %+v", list)
	}
}

func TestProvidersRejectsInvalidKey(t *testing.T) {
	ts, _ := newServer(t)
	cases := []string{"", "bad key", "../etc", "a.b", "a/b", strings.Repeat("x", 65)}
	for _, k := range cases {
		b, _ := json.Marshal(map[string]any{"op": "add", "key": k})
		resp, err := http.Post(ts.URL+"/api/providers", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("key %q: expected 400, got %d", k, resp.StatusCode)
		}
	}
}

// TestServeRespectsContextCancel ensures Ctrl-C / SIGTERM from the CLI layer
// reaches the HTTP server via ctx cancellation, instead of leaving
// ListenAndServe blocked forever.
func TestServeRespectsContextCancel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("model: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- Serve(ctx, p, addr) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve returned error after ctx cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return within 5s of ctx cancel")
	}
}

func TestProvidersModelsHappyPath(t *testing.T) {
	// Canned provider /models endpoint.
	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "aa"},
				{"id": "bb"},
			},
		})
	}))
	defer providerSrv.Close()

	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	yamlBody := []byte("providers:\n  test:\n    provider: openai\n    base_url: " + providerSrv.URL + "\n    api_key: sk-test\n    model: aa\n")
	if err := os.WriteFile(p, yamlBody, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"key": "test"})
	resp, err := http.Post(ts.URL+"/api/providers/models", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d: %s", resp.StatusCode, readBody(resp))
	}
	var out struct {
		Models []string `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Models) != 2 || out.Models[0] != "aa" || out.Models[1] != "bb" {
		t.Errorf("unexpected models: %+v", out.Models)
	}
}

func TestProvidersModelsUnsupportedType(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	// api_key must be "<key_id>.<secret>" for zhipu's signJWT to succeed; only
	// then does factory.New return a provider and we reach the ModelLister
	// type assertion (which is what this test exercises — zhipu does not
	// implement provider.ModelLister).
	os.WriteFile(p, []byte("providers:\n  test:\n    provider: zhipu\n    api_key: abc.def\n"), 0o644)
	s, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"key": "test"})
	resp, err := http.Post(ts.URL+"/api/providers/models", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	respBody := readBody(resp)
	if !strings.Contains(respBody, "does not support model listing") {
		t.Errorf("expected ModelLister rejection, got: %s", respBody)
	}
}

func TestProvidersModelsOriginCheck(t *testing.T) {
	ts, _ := newServer(t)
	body, _ := json.Marshal(map[string]string{"key": "test"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/providers/models", bytes.NewReader(body))
	req.Header.Set("Origin", "https://evil.example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
