package services

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func seedUser(t *testing.T, db *gorm.DB) *models.User {
	t.Helper()
	u := &models.User{Username: strPtr("alice"), Role: "default"}
	require.NoError(t, db.Create(u).Error)
	return u
}

func TestTempToken_IssueWithTTL_RespectsTTL(t *testing.T) {
	db := openTestDB(t)
	svc := NewTemporaryAuthTokenService(db)
	u := seedUser(t, db)

	tok, err := svc.IssueWithTTL(context.Background(), u.ID, 100*time.Millisecond)
	require.NoError(t, err)
	_, err = svc.Validate(context.Background(), tok)
	require.NoError(t, err)

	// Re-issue another and try the expired one
	tok2, _ := svc.IssueWithTTL(context.Background(), u.ID, time.Hour)
	time.Sleep(200 * time.Millisecond)
	_, err = svc.Validate(context.Background(), tok2) // tok2 still valid
	require.NoError(t, err)
}

func TestTempToken_IssueWithTTL_InvalidatesPriorTokens(t *testing.T) {
	db := openTestDB(t)
	svc := NewTemporaryAuthTokenService(db)
	u := seedUser(t, db)

	tok1, err := svc.IssueWithTTL(context.Background(), u.ID, time.Hour)
	require.NoError(t, err)

	tok2, err := svc.IssueWithTTL(context.Background(), u.ID, time.Hour)
	require.NoError(t, err)

	// tok1 should be invalidated (deleted)
	_, err = svc.Validate(context.Background(), tok1)
	require.Error(t, err)

	// tok2 should still be valid
	_, err = svc.Validate(context.Background(), tok2)
	require.NoError(t, err)
}

func TestTempToken_IssueWithTTL_TTLOutOfRange(t *testing.T) {
	db := openTestDB(t)
	svc := NewTemporaryAuthTokenService(db)
	u := seedUser(t, db)

	_, err := svc.IssueWithTTL(context.Background(), u.ID, 0)
	require.Error(t, err)

	_, err = svc.IssueWithTTL(context.Background(), u.ID, -1*time.Second)
	require.Error(t, err)

	_, err = svc.IssueWithTTL(context.Background(), u.ID, 2*time.Hour)
	require.Error(t, err)
}

func TestTempToken_Issue_BackwardCompatible(t *testing.T) {
	db := openTestDB(t)
	svc := NewTemporaryAuthTokenService(db)
	u := seedUser(t, db)

	tok, err := svc.Issue(context.Background(), u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	user, err := svc.Validate(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, u.ID, user.ID)
}
