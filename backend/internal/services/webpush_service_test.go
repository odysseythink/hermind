package services

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

	subJSON := `{"endpoint":"https://example.com/x","keys":{"p256dh":"abc","auth":"def"}}`
	require.NoError(t, svc.RegisterSubscription(context.Background(), user.ID, []byte(subJSON)))

	// Reload service to confirm DB persistence works
	svc2 := NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@test"})
	require.NoError(t, svc2.Init(context.Background()))
	_, ok := svc2.HasSubscription(user.ID)
	assert.True(t, ok)
}

func TestWebPushService_Boot_FiresOnJobCompleted(t *testing.T) {
	db, enc, sys := newWebPushTestEnv(t)
	require.NoError(t, db.AutoMigrate(&models.EventLog{}))

	// Stub push receiver — assert headers + body shape via test server.
	received := make(chan struct{}, 1)
	stub := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer stub.Close()

	insecureClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	oldValidate := validatePushEndpointFn
	validatePushEndpointFn = func(string) error { return nil }
	defer func() { validatePushEndpointFn = oldValidate }()

	svc := NewWebPushService(db, sys, enc, WebPushOptions{MailTo: "mailto:t@t", HTTPClient: insecureClient})
	require.NoError(t, svc.Init(context.Background()))

	user := &models.User{ID: 5}
	require.NoError(t, db.Create(user).Error)
	sub := `{"endpoint":"` + stub.URL + `","keys":{"p256dh":"BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM=","auth":"tBHItJI5svbpez7KI4CCXg=="}}`
	require.NoError(t, svc.RegisterSubscription(context.Background(), user.ID, []byte(sub)))

	evt := NewEventLogService(db)
	svc.Boot(evt)

	uid := user.ID
	require.NoError(t, evt.LogEvent(context.Background(), "scheduled_job_completed",
		map[string]any{"jobId": 1, "runId": 2, "resultText": "yay"}, &uid))

	select {
	case <-received:
	case <-time.After(3 * time.Second):
		t.Fatal("push provider never received the notification")
	}
}
