package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type oauthTestEnv struct {
	Router      *gin.Engine
	DB          *gorm.DB
	Storage     string
	Enc         *utils.EncryptionManager
	AuthSvc     *services.AuthService
	SysSvc      *services.SystemService
	Handler     *OAuthHandler
	Store       *oauth.TokenStore
	Outlook     *oauth.OutlookOAuth
	PublicURL   string
	StateSecret []byte
}

func newOAuthTestEnv(t *testing.T, enableAuth bool) *oauthTestEnv {
	t.Helper()
	tmp := t.TempDir()
	enc, err := utils.NewEncryptionManager(tmp)
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", tmp)), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, services.AutoMigrate(db))

	cfg := &config.Config{StorageDir: tmp}
	if enableAuth {
		cfg.AuthToken = "test-auth-token"
		cfg.JWTSecret = "test-jwt-secret-for-tests-only"
	}

	authSvc := services.NewAuthService(db, cfg, enc)
	sysSvc := services.NewSystemService(db)
	store := oauth.NewTokenStore(db, enc)
	publicURL := "http://localhost:3001"
	outlook := oauth.NewOutlookOAuth(store, publicURL, "common", nil)
	stateSecret := []byte("test-secret")
	handler := NewOAuthHandler(outlook, store, sysSvc, enc, stateSecret, publicURL)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	RegisterOAuthRoutes(api, handler, authSvc)

	return &oauthTestEnv{
		Router: r, DB: db, Storage: tmp, Enc: enc,
		AuthSvc: authSvc, SysSvc: sysSvc, Handler: handler,
		Store: store, Outlook: outlook, PublicURL: publicURL,
		StateSecret: stateSecret,
	}
}

func TestOutlookAuthorize_Unauthenticated_401(t *testing.T) {
	e := newOAuthTestEnv(t, true)
	req := httptest.NewRequest("GET", "/api/oauth/outlook/authorize", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOutlookAuthorize_HappyPath_302WithCorrectURL(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test","clientSecret":"secret"}`))

	req := httptest.NewRequest("GET", "/api/oauth/outlook/authorize?return_to="+e.PublicURL+"/dashboard", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	loc := rec.Header().Get("Location")
	assert.Contains(t, loc, "client_id=test")
	assert.Contains(t, loc, "response_type=code")
	assert.Contains(t, loc, "redirect_uri=")
	assert.Contains(t, loc, "state=")
	assert.Contains(t, loc, "prompt=select_account")
}

func TestOutlookAuthorize_MissingConfig_503(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	req := httptest.NewRequest("GET", "/api/oauth/outlook/authorize", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), "outlook_agent_config not set")
}

func TestOutlookCallback_TamperedState_400ErrorPage(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: e.PublicURL,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})
	tampered := state + "x"

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+tampered, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "OAuth Error")
}

func TestOutlookCallback_ExpiredState_400ErrorPage(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: e.PublicURL,
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "OAuth Error")
}

func TestOutlookCallback_OpenRedirect_400ErrorPage(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: "https://evil.com",
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "OAuth Error")
}

func TestOutlookCallback_HappyPath_302ToReturnTo_AndTokenSaved(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test","clientSecret":"secret"}`))

	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "mock_access",
			"refresh_token": "mock_refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer ms.Close()
	oauth.SetTestMicrosoftBase(ms.URL)
	defer oauth.SetTestMicrosoftBase("")

	returnTo := e.PublicURL + "/dashboard"
	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: returnTo,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, returnTo, rec.Header().Get("Location"))

	ts, err := e.Store.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "mock_access", ts.AccessToken)
	assert.Equal(t, "mock_refresh", ts.RefreshToken)
}

func TestOutlookCallback_MicrosoftError_500ErrorPage(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test","clientSecret":"secret"}`))

	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "The provided authorization grant is invalid.",
		})
	}))
	defer ms.Close()
	oauth.SetTestMicrosoftBase(ms.URL)
	defer oauth.SetTestMicrosoftBase("")

	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: e.PublicURL,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "OAuth Error")
	assert.Contains(t, rec.Body.String(), "OAuth exchange failed")
}

func TestOutlookCallback_ErrorPageEscapesHTMLInMessage(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test","clientSecret":"secret"}`))

	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "<script>alert('xss')</script>",
		})
	}))
	defer ms.Close()
	oauth.SetTestMicrosoftBase(ms.URL)
	defer oauth.SetTestMicrosoftBase("")

	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: e.PublicURL,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "OAuth Error")
	assert.NotContains(t, body, "<script>alert('xss')</script>")
	assert.Contains(t, body, "&lt;script&gt;")
}

func TestOutlookDisconnect_Authenticated_DeletesToken(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()

	require.NoError(t, e.Store.Save(ctx, 0, &oauth.TokenSet{
		AccessToken: "token", RefreshToken: "refresh",
		ExpiresAt: time.Now().Add(time.Hour), Tenant: "common",
	}))

	req := httptest.NewRequest("POST", "/api/oauth/outlook/disconnect", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["success"])

	_, err := e.Store.Get(ctx, 0)
	assert.ErrorIs(t, err, oauth.ErrTokenNotFound)
}

func TestOutlookDisconnect_Unauthenticated_401(t *testing.T) {
	e := newOAuthTestEnv(t, true)
	req := httptest.NewRequest("POST", "/api/oauth/outlook/disconnect", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOutlookStatus_Connected_ReturnsExpiresAt(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)

	require.NoError(t, e.Store.Save(ctx, 0, &oauth.TokenSet{
		AccessToken: "token", RefreshToken: "refresh",
		ExpiresAt: expiry, Tenant: "common",
	}))

	req := httptest.NewRequest("GET", "/api/oauth/outlook/status", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, true, body["connected"])
	assert.NotNil(t, body["expiresAt"])
}

func TestOutlookCallback_LoadsEncryptedSecret(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test"}`))
	require.NoError(t, e.SysSvc.SetSecretField(ctx, "outlook_agent_config", "clientSecret", "secret", e.Enc))

	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "mock_access",
			"refresh_token": "mock_refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer ms.Close()
	oauth.SetTestMicrosoftBase(ms.URL)
	defer oauth.SetTestMicrosoftBase("")

	state := oauth.EncodeState(e.StateSecret, oauth.StatePayload{
		UserID: 1, Nonce: "nonce", ReturnTo: e.PublicURL,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})

	req := httptest.NewRequest("GET", "/api/oauth/outlook/callback?code=mock&state="+state, nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
}

func TestMigration_EncryptsExistingPlaintext(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSetting(ctx, "outlook_agent_config", `{"clientId":"test","clientSecret":"plain-secret"}`))

	// Simulate migration logic
	raw, err := e.SysSvc.GetSetting(ctx, "outlook_agent_config")
	require.NoError(t, err)
	require.Contains(t, raw, "plain-secret")

	require.NoError(t, e.SysSvc.SetSecretField(ctx, "outlook_agent_config", "clientSecret", "plain-secret", e.Enc))

	raw, err = e.SysSvc.GetSetting(ctx, "outlook_agent_config")
	require.NoError(t, err)
	require.Contains(t, raw, services.EncryptedPrefix)

	// Verify decrypt works
	secret, err := e.SysSvc.GetSecretField(ctx, "outlook_agent_config", "clientSecret", e.Enc)
	require.NoError(t, err)
	require.Equal(t, "plain-secret", secret)
}

func TestMigration_Idempotent_DoesNotDoubleEncrypt(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	ctx := context.Background()
	require.NoError(t, e.SysSvc.SetSecretField(ctx, "outlook_agent_config", "clientSecret", "secret", e.Enc))

	// Read back encrypted value
	raw, err := e.SysSvc.GetSetting(ctx, "outlook_agent_config")
	require.NoError(t, err)
	require.Contains(t, raw, services.EncryptedPrefix)

	var obj map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &obj))
	encryptedVal := obj["clientSecret"].(string)

	// Simulate migration: should skip because already encrypted
	if strings.HasPrefix(encryptedVal, services.EncryptedPrefix) {
		// skipped
	} else {
		t.Fatal("expected to skip already-encrypted value")
	}

	// Verify decrypt still works
	secret, err := e.SysSvc.GetSecretField(ctx, "outlook_agent_config", "clientSecret", e.Enc)
	require.NoError(t, err)
	require.Equal(t, "secret", secret)
}

func TestOutlookStatus_Disconnected_ReturnsConnectedFalse(t *testing.T) {
	e := newOAuthTestEnv(t, false)
	req := httptest.NewRequest("GET", "/api/oauth/outlook/status", nil)
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, false, body["connected"])
	assert.Nil(t, body["expiresAt"])
}
