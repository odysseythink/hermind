// provider/wenxin/oauth.go
package wenxin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// oauthResponse matches the Baidu OAuth token endpoint response body.
type oauthResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"` // seconds
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// fetchAccessToken performs an OAuth 2.0 client_credentials flow against
// Baidu's token endpoint and returns the access_token.
func (w *Wenxin) fetchAccessToken(ctx context.Context) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "client_credentials")
	params.Set("client_id", w.apiKey)
	params.Set("client_secret", w.secretKey)

	fullURL := w.oauthURL + "?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("wenxin oauth: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := w.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("wenxin oauth: network: %w", err)
	}
	defer resp.Body.Close()

	var body oauthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("wenxin oauth: decode: %w", err)
	}

	if body.Error != "" {
		return "", fmt.Errorf("wenxin oauth: %s: %s", body.Error, body.ErrorDescription)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("wenxin oauth: no access_token in response")
	}

	// Cache with a 5-minute safety margin before the real expiry.
	w.tokenMu.Lock()
	w.token = body.AccessToken
	w.tokenExp = time.Now().Add(time.Duration(body.ExpiresIn-300) * time.Second)
	w.tokenMu.Unlock()

	return body.AccessToken, nil
}

// getAccessToken returns a cached token if it's still valid, otherwise
// fetches a new one.
func (w *Wenxin) getAccessToken(ctx context.Context) (string, error) {
	w.tokenMu.Lock()
	if w.token != "" && time.Now().Before(w.tokenExp) {
		token := w.token
		w.tokenMu.Unlock()
		return token, nil
	}
	w.tokenMu.Unlock()

	return w.fetchAccessToken(ctx)
}
