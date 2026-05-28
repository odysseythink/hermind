package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWhitelistTestDB(t *testing.T) *SystemService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}))
	return NewSystemService(db)
}

func TestWhitelistSvc_Add_Get_RoundTrip(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()
	uid := 42

	require.NoError(t, svc.Add(ctx, &uid, "mcp-test-tool"))
	list, err := svc.Get(ctx, &uid)
	require.NoError(t, err)
	require.Equal(t, []string{"mcp-test-tool"}, list)
}

func TestWhitelistSvc_LabelDifferent_PerUserVsSingleUser(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()

	// Single-user (nil userID)
	require.NoError(t, svc.Add(ctx, nil, "tool-a"))
	single, err := svc.Get(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"tool-a"}, single)

	// User 7 should not see single-user list
	uid := 7
	userList, err := svc.Get(ctx, &uid)
	require.NoError(t, err)
	require.Empty(t, userList)
}

func TestWhitelistSvc_IsWhitelisted_True(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()
	uid := 1

	require.NoError(t, svc.Add(ctx, &uid, "rag-memory"))
	require.True(t, svc.IsWhitelisted(ctx, &uid, "rag-memory"))
}

func TestWhitelistSvc_IsWhitelisted_False(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()
	uid := 1

	require.False(t, svc.IsWhitelisted(ctx, &uid, "not-there"))
}

func TestWhitelistSvc_ClearSingleUser(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()

	require.NoError(t, svc.Add(ctx, nil, "tool-a"))
	require.NoError(t, svc.ClearSingleUser(ctx))
	list, err := svc.Get(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestWhitelistSvc_Add_Idempotent(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()
	uid := 1

	require.NoError(t, svc.Add(ctx, &uid, "tool-x"))
	require.NoError(t, svc.Add(ctx, &uid, "tool-x"))
	list, err := svc.Get(ctx, &uid)
	require.NoError(t, err)
	require.Equal(t, []string{"tool-x"}, list)
}

func TestWhitelistSvc_Remove(t *testing.T) {
	sysSvc := newWhitelistTestDB(t)
	svc := NewAgentSkillWhitelistService(sysSvc)
	ctx := context.Background()
	uid := 1

	require.NoError(t, svc.Add(ctx, &uid, "keep"))
	require.NoError(t, svc.Add(ctx, &uid, "remove"))
	require.NoError(t, svc.Remove(ctx, &uid, "remove"))
	list, err := svc.Get(ctx, &uid)
	require.NoError(t, err)
	require.Equal(t, []string{"keep"}, list)
}
