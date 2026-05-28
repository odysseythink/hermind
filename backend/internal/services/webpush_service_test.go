package services

import (
	"context"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWebPushTestEnv(t *testing.T) (*gorm.DB, *utils.EncryptionManager, *SystemService) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SystemSetting{}, &models.User{}))
	enc, err := utils.NewEncryptionManager(t.TempDir())
	require.NoError(t, err)
	return db, enc, NewSystemService(db)
}

func TestWebPushService_InitGeneratesVAPID(t *testing.T) {
	db, enc, sys := newWebPushTestEnv(t)
	svc := NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@test"})
	require.NoError(t, svc.Init(context.Background()))
	assert.NotEmpty(t, svc.PublicVAPIDKey())
	// Second Init must reuse keys.
	pub := svc.PublicVAPIDKey()
	require.NoError(t, NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@test"}).Init(context.Background()))
	require.NoError(t, svc.Init(context.Background()))
	assert.Equal(t, pub, svc.PublicVAPIDKey())
}

func TestWebPushService_RegisterAndLoad(t *testing.T) {
	db, enc, sys := newWebPushTestEnv(t)
	svc := NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@test"})
	require.NoError(t, svc.Init(context.Background()))

	user := &models.User{ID: 7}
	require.NoError(t, db.Create(user).Error)

	subJSON := `{"endpoint":"https://example/x","keys":{"p256dh":"abc","auth":"def"}}`
	require.NoError(t, svc.RegisterSubscription(context.Background(), user.ID, []byte(subJSON)))

	// Reload service to confirm DB persistence works
	svc2 := NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@test"})
	require.NoError(t, svc2.Init(context.Background()))
	_, ok := svc2.HasSubscription(user.ID)
	assert.True(t, ok)
}
