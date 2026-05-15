package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleRender(t *testing.T) {
	body, _ := json.Marshal(map[string]string{
		"content": "# Hello\n\n`code`",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/render", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleRender(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp renderResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !strings.Contains(resp.HTML, "<h1>Hello</h1>") {
		t.Fatalf("expected h1 in HTML, got: %s", resp.HTML)
	}
}
