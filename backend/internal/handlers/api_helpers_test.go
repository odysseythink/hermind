package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestApiV1RequireMultiUser_Allows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	cfg := &config.Config{MultiUserMode: true}
	assert.True(t, apiV1RequireMultiUser(c, cfg))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestApiV1RequireMultiUser_Denies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	cfg := &config.Config{MultiUserMode: false}
	assert.False(t, apiV1RequireMultiUser(c, cfg))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Multi-User mode")
}
