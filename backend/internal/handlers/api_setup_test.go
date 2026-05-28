package handlers

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type apiTestEnv struct {
	Router    *gin.Engine
	DB        *gorm.DB
	Cfg       *config.Config
	APIKeySvc *services.APIKeyService
	APIKey    string // raw secret to put in Authorization: Bearer <APIKey>
}

func newAPITestEnv(t *testing.T, cfg *config.Config) *apiTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, services.AutoMigrate(db))
	if cfg == nil {
		cfg = &config.Config{MultiUserMode: true}
	}
	keySvc := services.NewAPIKeyService(db)
	key, err := keySvc.Create(context.Background(), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, key.Secret)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	return &apiTestEnv{
		Router:    r,
		DB:        db,
		Cfg:       cfg,
		APIKeySvc: keySvc,
		APIKey:    *key.Secret,
	}
}
