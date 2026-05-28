package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestFileSystemAgentAvailable_ReadsConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{AgentFilesystemEnabled: tc.enabled}
			h := NewAgentSkillHandler(nil, cfg)
			r := gin.New()
			r.GET("/test", h.FileSystemAgentAvailable)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			var body map[string]bool
			assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["available"])
		})
	}
}

func TestCreateFilesAgentAvailable_ReadsConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AgentCreateFilesEnabled: true}
	h := NewAgentSkillHandler(nil, cfg)
	r := gin.New()
	r.GET("/test", h.CreateFilesAgentAvailable)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]bool
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body["available"])
}
