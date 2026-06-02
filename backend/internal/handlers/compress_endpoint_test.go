package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompressEndpoint_NothingToCompress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))

	ws := &models.Workspace{Name: "test", Slug: "test-slug"}
	require.NoError(t, db.Create(ws).Error)

	compStore := agentcompression.NewCompactionStore(db)
	sysSvc := services.NewSystemService(db)
	require.NoError(t, sysSvc.SetSetting(context.Background(), "context_compress_enabled", "true"))

	chatSvc := services.NewChatService(db, nil, nil, nil, nil, nil, nil, nil, nil, compStore, sysSvc)
	h := NewChatHandler(chatSvc)

	reqBody, _ := json.Marshal(dto.CompressRequest{Topic: "summary"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/workspace/test-slug/compress", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("workspace", ws)

	h.Compress(c)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "nothing to compress")
}
