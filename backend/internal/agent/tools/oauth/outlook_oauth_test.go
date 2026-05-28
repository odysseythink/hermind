package oauth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/stretchr/testify/require"
)

func TestOutlookOAuth_AuthorizeURL_ContainsRequiredParams(t *testing.T) {
	store := oauth.NewTokenStore(nil, nil)
	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	state := oauth.EncodeState([]byte("secret"), oauth.StatePayload{
		UserID:    1,
		Nonce:     "n",
		ReturnTo:  "https://app.example.com/dash",
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	})
	u := o.AuthorizeURL(state, "my-client-id", "")
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	q := parsed.Query()
	require.Equal(t, "my-client-id", q.Get("client_id"))
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, "https://app.example.com/api/oauth/outlook/callback", q.Get("redirect_uri"))
	require.Equal(t, "query", q.Get("response_mode"))
	require.Equal(t, "offline_access Mail.Read Mail.ReadWrite Mail.Send User.Read", q.Get("scope"))
	require.Equal(t, state, q.Get("state"))
	require.Equal(t, "select_account", q.Get("prompt"))
	require.True(t, strings.HasPrefix(u, "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
}

func TestOutlookOAuth_ExchangeCode_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/common/oauth2/v2.0/token", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, "my-code", r.PostFormValue("code"))
		require.Equal(t, "authorization_code", r.PostFormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	store := oauth.NewTokenStore(nil, nil)
	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	ts, err := o.ExchangeCode(context.Background(), "my-code", "client-id", "client-secret", "")
	require.NoError(t, err)
	require.Equal(t, "new-access", ts.AccessToken)
	require.Equal(t, "new-refresh", ts.RefreshToken)
	require.Equal(t, "common", ts.Tenant)
	require.WithinDuration(t, time.Now().Add(3600*time.Second-60*time.Second), ts.ExpiresAt, 5*time.Second)
}

func TestOutlookOAuth_ExchangeCode_BadStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "The provided authorization grant is invalid.",
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	store := oauth.NewTokenStore(nil, nil)
	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	_, err := o.ExchangeCode(context.Background(), "bad-code", "client-id", "client-secret", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "microsoft oauth error")
}

func TestOutlookOAuth_ValidAccessToken_NotExpired_NoRefresh(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "valid-access",
		RefreshToken: "valid-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		Tenant:       "common",
	}))

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	tok, err := o.ValidAccessToken(ctx, 42, "client-id", "client-secret")
	require.NoError(t, err)
	require.Equal(t, "valid-access", tok)
}

func TestOutlookOAuth_ValidAccessToken_Expired_TriggersRefresh(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/common/oauth2/v2.0/token", r.URL.Path)
		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, "old-refresh", r.PostFormValue("refresh_token"))
		require.Equal(t, "refresh_token", r.PostFormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-access",
			"refresh_token": "refreshed-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	tok, err := o.ValidAccessToken(ctx, 42, "client-id", "client-secret")
	require.NoError(t, err)
	require.Equal(t, "refreshed-access", tok)

	// Verify store was updated.
	ts, err := store.Get(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, "refreshed-access", ts.AccessToken)
	require.Equal(t, "refreshed-refresh", ts.RefreshToken)
}

func TestOutlookOAuth_ValidAccessToken_RefreshMissingNewRefreshToken_PreservesOld(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "precious-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "refreshed-access",
			"expires_in":   3600,
			"token_type":   "Bearer",
			// No refresh_token in response.
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	tok, err := o.ValidAccessToken(ctx, 42, "client-id", "client-secret")
	require.NoError(t, err)
	require.Equal(t, "refreshed-access", tok)

	ts, err := store.Get(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, "precious-refresh", ts.RefreshToken)
}

func TestOutlookOAuth_ValidAccessToken_RefreshFails_ReturnsError(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "Refresh token expired.",
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)
	_, err := o.ValidAccessToken(ctx, 42, "client-id", "client-secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "refresh")
}

func TestValidAccessToken_ConcurrentExpired_RefreshOnceViaDBLock(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	var refreshCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		time.Sleep(50 * time.Millisecond) // simulate network latency to amplify race
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  fmt.Sprintf("new-token-%d", refreshCount.Load()),
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok, err := o.ValidAccessToken(context.Background(), 42, "cid", "secret")
			require.NoError(t, err)
			require.NotEmpty(t, tok)
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), refreshCount.Load(), "must refresh exactly once")
}

func TestOutlookOAuth_ConcurrentValidAccessToken_RefreshOnce(t *testing.T) {
	db, enc := newTokenStoreTestDB(t)
	store := oauth.NewTokenStore(db, enc)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, 42, &oauth.TokenSet{
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Tenant:       "common",
	}))

	var refreshCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "concurrent-access",
			"refresh_token": "concurrent-refresh",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer srv.Close()
	oauth.SetTestMicrosoftBase(srv.URL)
	defer oauth.SetTestMicrosoftBase("")

	o := oauth.NewOutlookOAuth(store, "https://app.example.com", "common", nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = o.ValidAccessToken(ctx, 42, "client-id", "client-secret")
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), refreshCount.Load())
}
