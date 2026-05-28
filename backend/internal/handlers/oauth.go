package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
)

type OAuthHandler struct {
	outlook       *oauth.OutlookOAuth
	store         *oauth.TokenStore
	sysSvc        *services.SystemService
	enc           *utils.EncryptionManager
	stateSecret   []byte
	publicBaseURL string
}

func NewOAuthHandler(outlook *oauth.OutlookOAuth, store *oauth.TokenStore, sysSvc *services.SystemService, enc *utils.EncryptionManager, stateSecret []byte, publicBaseURL string) *OAuthHandler {
	return &OAuthHandler{outlook: outlook, store: store, sysSvc: sysSvc, enc: enc, stateSecret: stateSecret, publicBaseURL: publicBaseURL}
}

func RegisterOAuthRoutes(r *gin.RouterGroup, h *OAuthHandler, authSvc *services.AuthService) {
	g := r.Group("/oauth/outlook")
	g.GET("/authorize", middleware.ValidatedRequest(authSvc), h.OutlookAuthorize)
	g.GET("/callback", h.OutlookCallback) // state-self-authenticated
	g.POST("/disconnect", middleware.ValidatedRequest(authSvc), h.OutlookDisconnect)
	g.GET("/status", middleware.ValidatedRequest(authSvc), h.OutlookStatus)
}

type outlookConfig struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Tenant       string `json:"tenant,omitempty"`
}

func (h *OAuthHandler) loadConfig(ctx context.Context) (outlookConfig, error) {
	raw, err := h.sysSvc.GetSetting(ctx, "outlook_agent_config")
	if err != nil || raw == "" {
		return outlookConfig{}, errors.New("outlook_agent_config not set")
	}
	var c outlookConfig
	_ = json.Unmarshal([]byte(raw), &c)
	// clientSecret may be encrypted
	secret, err := h.sysSvc.GetSecretField(ctx, "outlook_agent_config", "clientSecret", h.enc)
	if err != nil {
		return c, fmt.Errorf("decrypt clientSecret: %w", err)
	}
	c.ClientSecret = secret
	if c.ClientID == "" || c.ClientSecret == "" {
		return c, errors.New("outlook_agent_config incomplete")
	}
	return c, nil
}

func (h *OAuthHandler) OutlookAuthorize(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	cfg, err := h.loadConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	returnTo := c.Query("return_to")
	if returnTo == "" {
		returnTo = h.publicBaseURL
	}
	nonce := newNonce()
	state := oauth.EncodeState(h.stateSecret, oauth.StatePayload{
		UserID: user.ID, Nonce: nonce, ReturnTo: returnTo,
		ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
	})
	url := h.outlook.AuthorizeURL(state, cfg.ClientID, cfg.Tenant)
	c.Redirect(http.StatusFound, url)
}

func (h *OAuthHandler) OutlookCallback(c *gin.Context) {
	code := c.Query("code")
	encState := c.Query("state")
	if code == "" || encState == "" {
		h.errorPage(c, http.StatusBadRequest, "Missing code or state")
		return
	}
	state, err := oauth.DecodeState(h.stateSecret, encState, h.publicBaseURL)
	if err != nil {
		h.errorPage(c, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := h.loadConfig(c.Request.Context())
	if err != nil {
		h.errorPage(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	ts, err := h.outlook.ExchangeCode(c.Request.Context(), code, cfg.ClientID, cfg.ClientSecret, cfg.Tenant)
	if err != nil {
		h.errorPage(c, http.StatusInternalServerError, "OAuth exchange failed: "+err.Error())
		return
	}
	if err := h.store.Save(c.Request.Context(), state.UserID, ts); err != nil {
		h.errorPage(c, http.StatusInternalServerError, "Failed to save token: "+err.Error())
		return
	}
	c.Redirect(http.StatusFound, state.ReturnTo)
}

func (h *OAuthHandler) OutlookDisconnect(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	if err := h.store.Delete(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *OAuthHandler) OutlookStatus(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	ts, err := h.store.Get(c.Request.Context(), user.ID)
	if errors.Is(err, oauth.ErrTokenNotFound) {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": true, "expiresAt": ts.ExpiresAt})
}

// errorPage renders a minimal HTML error page with HTML-escaped message.
func (h *OAuthHandler) errorPage(c *gin.Context, status int, message string) {
	safe := template.HTMLEscapeString(message)
	html := `<!doctype html><html><body>
<h1>OAuth Error</h1>
<p>` + safe + `</p>
<p>Please close this window and try again.</p>
</body></html>`
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(status, html)
}

func newNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
