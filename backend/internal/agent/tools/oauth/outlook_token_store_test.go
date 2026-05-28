package oauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTokenStoreTestDB(t *testing.T) (*gorm.DB, *utils.EncryptionManager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OutlookOAuthToken{}))
	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)
	return db, enc
}

func TestTokenStore_SaveGet_RoundTrip(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	ts := &oauth.TokenSet{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}
	require.NoError(t, store.Save(ctx, 42, ts))

	got, err := store.Get(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, ts.AccessToken, got.AccessToken)
	require.Equal(t, ts.RefreshToken, got.RefreshToken)
	require.Equal(t, ts.Tenant, got.Tenant)
	require.WithinDuration(t, ts.ExpiresAt, got.ExpiresAt, time.Second)
}

func TestTokenStore_Save_OverwritesExisting(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		Tenant:       "organizations",
	}))

	got, err := store.Get(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, "new-access", got.AccessToken)
	require.Equal(t, "new-refresh", got.RefreshToken)
	require.Equal(t, "organizations", got.Tenant)
}

func TestTokenStore_Get_NotFound_ReturnsErrTokenNotFound(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	_, err := store.Get(ctx, 999)
	require.Error(t, err)
	require.ErrorIs(t, err, oauth.ErrTokenNotFound)
}

func TestTokenStore_Delete_Idempotent(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))
	require.NoError(t, store.Delete(ctx, 42))
	_, err := store.Get(ctx, 42)
	require.ErrorIs(t, err, oauth.ErrTokenNotFound)

	// Deleting again should not error.
	require.NoError(t, store.Delete(ctx, 42))
}

func TestTokenStore_AccessTokenAndRefreshAreEncryptedAtRest(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	plainAT := "sensitive-access-token"
	plainRT := "sensitive-refresh-token"
	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  plainAT,
		RefreshToken: plainRT,
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	var row models.OutlookOAuthToken
	require.NoError(t, db.First(&row, "user_id = ?", 42).Error)
	require.NotEqual(t, plainAT, row.EncryptedAccessToken)
	require.NotEqual(t, plainRT, row.EncryptedRefreshToken)
	require.NotEmpty(t, row.EncryptedAccessToken)
	require.NotEmpty(t, row.EncryptedRefreshToken)
}
