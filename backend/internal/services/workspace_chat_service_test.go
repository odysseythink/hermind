package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWorkspaceChatTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(
		&models.User{},
		&models.Workspace{},
		&models.WorkspaceChat{},
	)
	require.NoError(t, err)
	return db
}

func seedWorkspaceAndChat(t *testing.T, db *gorm.DB) (workspaceID int, userID int) {
	ws := models.Workspace{Name: "Test WS", Slug: "test-ws"}
	require.NoError(t, db.Create(&ws).Error)
	workspaceID = ws.ID

	username := "testuser"
	user := models.User{Username: &username, Role: "admin"}
	require.NoError(t, db.Create(&user).Error)
	userID = user.ID

	resp, _ := json.Marshal(map[string]string{"text": "hello world"})
	chat := models.WorkspaceChat{
		WorkspaceID: workspaceID,
		Prompt:      "hi",
		Response:    string(resp),
		UserID:      &userID,
		Include:     true,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, db.Create(&chat).Error)
	return
}

func TestWorkspaceChatService_ListChats(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	wid, uid := seedWorkspaceAndChat(t, db)
	_ = wid
	_ = uid

	chats, total, err := svc.ListChats(ctx, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, chats, 1)
	assert.Equal(t, "Test WS", chats[0].Workspace.Name)
	assert.Equal(t, "testuser", chats[0].User.Username)
}

func TestWorkspaceChatService_ListChats_Pagination(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	ws := models.Workspace{Name: "WS", Slug: "ws"}
	require.NoError(t, db.Create(&ws).Error)

	for i := 0; i < 5; i++ {
		chat := models.WorkspaceChat{
			WorkspaceID: ws.ID,
			Prompt:      "msg",
			Response:    `{}`,
			CreatedAt:   time.Now(),
		}
		require.NoError(t, db.Create(&chat).Error)
	}

	chats, total, err := svc.ListChats(ctx, 0, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, chats, 2)
}

func TestWorkspaceChatService_DeleteChat(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	wid, uid := seedWorkspaceAndChat(t, db)
	_ = wid
	_ = uid

	var before models.WorkspaceChat
	require.NoError(t, db.First(&before).Error)

	err := svc.DeleteChat(ctx, before.ID)
	require.NoError(t, err)

	var after models.WorkspaceChat
	assert.Error(t, db.First(&after).Error)
}

func TestWorkspaceChatService_DeleteAllChats(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	ws := models.Workspace{Name: "WS", Slug: "ws"}
	require.NoError(t, db.Create(&ws).Error)

	for i := 0; i < 3; i++ {
		chat := models.WorkspaceChat{
			WorkspaceID: ws.ID,
			Prompt:      "msg",
			Response:    `{}`,
			CreatedAt:   time.Now(),
		}
		require.NoError(t, db.Create(&chat).Error)
	}

	err := svc.DeleteAllChats(ctx)
	require.NoError(t, err)

	var count int64
	db.Model(&models.WorkspaceChat{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestWorkspaceChatService_ExportChats_CSV(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	seedWorkspaceAndChat(t, db)

	ct, data, err := svc.ExportChats(ctx, "csv")
	require.NoError(t, err)
	assert.Equal(t, "text/csv", ct)
	assert.Contains(t, string(data), "id,workspace,prompt,response,sent_at,username,rating")
	assert.Contains(t, string(data), "hello world")
}

func TestWorkspaceChatService_ExportChats_JSONL(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	seedWorkspaceAndChat(t, db)

	ct, data, err := svc.ExportChats(ctx, "jsonl")
	require.NoError(t, err)
	assert.Equal(t, "application/jsonl", ct)
	assert.Contains(t, string(data), "messages")
	assert.Contains(t, string(data), "system")
	assert.Contains(t, string(data), "user")
	assert.Contains(t, string(data), "assistant")
}

func TestWorkspaceChatService_ExportChats_JSON(t *testing.T) {
	db := setupWorkspaceChatTestDB(t)
	svc := NewWorkspaceChatService(db)
	ctx := context.Background()

	seedWorkspaceAndChat(t, db)

	ct, data, err := svc.ExportChats(ctx, "json")
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)
	assert.Contains(t, string(data), "hello world")
}
