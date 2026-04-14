package security

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MCPOAuthClient performs the OAuth 2.1 client_credentials grant
// against an MCP server's token endpoint.
type MCPOAuthClient struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scope        string
	http         *http.Client
}

func NewMCPOAuthClient(tokenURL, clientID, clientSecret, scope string) *MCPOAuthClient {
	return &MCPOAuthClient{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        scope,
		http:         &http.Client{Timeout: 15 * time.Second},
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// FetchToken returns a fresh bearer access token.
func (c *MCPOAuthClient) FetchToken(ctx context.Context) (string, error) {
	if c.TokenURL == "" || c.ClientID == "" || c.ClientSecret == "" {
		return "", fmt.Errorf("mcp_oauth: missing token_url/client_id/client_secret")
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	if c.Scope != "" {
		form.Set("scope", c.Scope)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("mcp_oauth: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("mcp_oauth: status %d: %s", resp.StatusCode, string(body))
	}
	var out tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("mcp_oauth: empty access_token in response")
	}
	return out.AccessToken, nil
}
