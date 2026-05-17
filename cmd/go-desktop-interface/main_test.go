package main

import (
	"testing"
)

func TestHandleRequestSmoke(t *testing.T) {
	// Skip if server not initialized (e.g., no config)
	if globalServer == nil {
		t.Skip("server not initialized — run with valid hermind config")
	}

	tests := []struct {
		method string
		path   string
		wantOK bool
	}{
		{"GET", "/health", true},
		{"GET", "/api/status", true},
		{"GET", "/api/config", true},
		{"GET", "/api/providers", true},
		{"GET", "/api/tools", true},
		{"GET", "/api/skills", true},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			resp := handleRequest(tt.method, tt.path, nil)
			if resp.OK != tt.wantOK {
				t.Errorf("handleRequest(%q, %q) OK=%v want %v, error=%s",
					tt.method, tt.path, resp.OK, tt.wantOK, resp.Error)
			}
		})
	}
}
