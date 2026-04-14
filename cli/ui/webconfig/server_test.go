package webconfig

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
	resp, _ := http.Get(ts.URL + "/api/config")
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
