package memprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPJSONSuccess(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("missing content-type")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var out struct {
		OK bool `json:"ok"`
	}
	err := httpJSONWith(context.Background(), srv.Client(), "POST", srv.URL, "abc",
		map[string]string{"hello": "world"}, &out)
	if err != nil {
		t.Fatalf("httpJSONWith: %v", err)
	}
	if !out.OK {
		t.Errorf("expected ok=true, got %+v", out)
	}
	if got["hello"] != "world" {
		t.Errorf("server did not see body: %+v", got)
	}
}

func TestHTTPJSONErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := httpJSONWith(context.Background(), srv.Client(), "GET", srv.URL, "", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error did not include body: %v", err)
	}
}
