package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	microsoftAuthBase = "https://login.microsoftonline.com"
	outlookScopes     = "offline_access Mail.Read Mail.ReadWrite Mail.Send User.Read"
	tokenExpiryLeeway = 60 * time.Second
)

// testMicrosoftBase overrides login.microsoftonline.com in tests.
var testMicrosoftBase string

func SetTestMicrosoftBase(u string) { testMicrosoftBase = u }

type OutlookOAuth struct {
	store       *TokenStore
	redirectURI string
	defaultAuth string
	http        *http.Client
	refreshMu   sync.Mutex
}

func NewOutlookOAuth(store *TokenStore, publicBaseURL, defaultAuthority string, httpClient *http.Client) *OutlookOAuth {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &OutlookOAuth{
		store:       store,
		redirectURI: publicBaseURL + "/api/oauth/outlook/callback",
		defaultAuth: defaultAuthority,
		http:        httpClient,
	}
}

func (o *OutlookOAuth) authBase() string {
	if testMicrosoftBase != "" {
		return testMicrosoftBase
	}
	return microsoftAuthBase
}

func (o *OutlookOAuth) RedirectURI() string { return o.redirectURI }

func (o *OutlookOAuth) AuthorizeURL(state, clientID, authority string) string {
	if authority == "" {
		authority = o.defaultAuth
	}
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", o.redirectURI)
	v.Set("response_mode", "query")
	v.Set("scope", outlookScopes)
	v.Set("state", state)
	v.Set("prompt", "select_account")
	return fmt.Sprintf("%s/%s/oauth2/v2.0/authorize?%s", o.authBase(), authority, v.Encode())
}

func (o *OutlookOAuth) ExchangeCode(ctx context.Context, code, clientID, clientSecret, authority string) (*TokenSet, error) {
	if authority == "" {
		authority = o.defaultAuth
	}
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", o.redirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("scope", outlookScopes)
	return o.tokenPOST(ctx, authority, form)
}

func (o *OutlookOAuth) refresh(ctx context.Context, refreshToken, clientID, clientSecret, authority string) (*TokenSet, error) {
	if authority == "" {
		authority = o.defaultAuth
	}
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")
	form.Set("scope", outlookScopes)
	return o.tokenPOST(ctx, authority, form)
}

type msTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (o *OutlookOAuth) tokenPOST(ctx context.Context, authority string, form url.Values) (*TokenSet, error) {
	url := fmt.Sprintf("%s/%s/oauth2/v2.0/token", o.authBase(), authority)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token endpoint: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	var data msTokenResp
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("token response not JSON: %w (body=%s)", err, body)
	}
	if data.Error != "" {
		return nil, fmt.Errorf("microsoft oauth error: %s: %s", data.Error, data.ErrorDesc)
	}
	if data.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token (status=%d)", resp.StatusCode)
	}
	return &TokenSet{
		AccessToken:  data.AccessToken,
		RefreshToken: data.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(data.ExpiresIn)*time.Second - tokenExpiryLeeway),
		Tenant:       authority,
	}, nil
}

func (o *OutlookOAuth) ValidAccessToken(ctx context.Context, userID int, clientID, clientSecret string) (string, error) {
	o.refreshMu.Lock()
	defer o.refreshMu.Unlock()

	var accessToken string
	err := o.store.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.OutlookOAuthToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTokenNotFound
		}
		if err != nil {
			return err
		}

		// Re-check expiry inside the transaction — another process may have refreshed.
		if time.Now().Before(row.ExpiresAt) {
			pt, err := o.store.enc.Decrypt(row.EncryptedAccessToken)
			if err != nil {
				return err
			}
			accessToken = pt
			return nil
		}

		oldRT, err := o.store.enc.Decrypt(row.EncryptedRefreshToken)
		if err != nil {
			return err
		}
		newTS, err := o.refresh(ctx, oldRT, clientID, clientSecret, row.Tenant)
		if err != nil {
			return fmt.Errorf("refresh: %w", err)
		}
		if newTS.RefreshToken == "" {
			newTS.RefreshToken = oldRT
		}
		if newTS.Tenant == "" {
			newTS.Tenant = row.Tenant
		}

		atEnc, err := o.store.enc.Encrypt(newTS.AccessToken)
		if err != nil {
			return err
		}
		rtEnc, err := o.store.enc.Encrypt(newTS.RefreshToken)
		if err != nil {
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{
			"encrypted_access_token":  atEnc,
			"encrypted_refresh_token": rtEnc,
			"expires_at":              newTS.ExpiresAt,
			"tenant":                  newTS.Tenant,
		}).Error; err != nil {
			return err
		}

		accessToken = newTS.AccessToken
		return nil
	})
	return accessToken, err
}
