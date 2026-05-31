package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type BrowserExtensionHandler struct {
	extSvc  *services.BrowserExtensionService
	wsSvc   *services.WorkspaceService
	docSvc  *services.DocumentService
	authSvc *services.AuthService
	cfg     *config.Config
}

func NewBrowserExtensionHandler(
	extSvc *services.BrowserExtensionService,
	wsSvc *services.WorkspaceService,
	docSvc *services.DocumentService,
	authSvc *services.AuthService,
	cfg *config.Config,
) *BrowserExtensionHandler {
	return &BrowserExtensionHandler{extSvc: extSvc, wsSvc: wsSvc, docSvc: docSvc, authSvc: authSvc, cfg: cfg}
}

// --- Extension-facing endpoints (brx auth) ---

func (h *BrowserExtensionHandler) Check(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	apiKey := c.MustGet("apiKey").(*models.BrowserExtensionApiKey)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"connected":  true,
		"workspaces": workspaces,
		"apiKeyId":   apiKey.ID,
	})
}

func (h *BrowserExtensionHandler) Disconnect(c *gin.Context) {
	apiKey := c.MustGet("apiKey").(*models.BrowserExtensionApiKey)
	if err := h.extSvc.DeleteKey(c.Request.Context(), apiKey.ID, nil, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Browser extension disconnected"})
}

func (h *BrowserExtensionHandler) Workspaces(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	workspaces, err := h.wsSvc.List(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *BrowserExtensionHandler) EmbedContent(c *gin.Context) {
	var req struct {
		WorkspaceID int               `json:"workspaceId" binding:"required"`
		TextContent string            `json:"textContent" binding:"required"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	ws, err := h.wsSvc.GetByID(c.Request.Context(), req.WorkspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	title := req.Metadata["title"]
	if title == "" {
		title = "Browser Extension Embed"
	}
	meta := map[string]any{
		"title":  title,
		"url":    req.Metadata["url"],
		"source": "browser-extension",
	}

	docs, err := h.docSvc.SaveRawText(c.Request.Context(), req.TextContent, title, meta, []string{ws.Slug})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	for _, doc := range docs {
		go func(d *models.WorkspaceDocument) {
			defer func() {
				_ = recover()
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			_ = h.docSvc.EmbedDocument(ctx, d)
		}(doc)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Content embedded"})
}

func (h *BrowserExtensionHandler) UploadContent(c *gin.Context) {
	var req struct {
		TextContent string            `json:"textContent" binding:"required"`
		Metadata    map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	title := req.Metadata["title"]
	if title == "" {
		title = "Browser Extension Upload"
	}
	meta := map[string]any{
		"title":  title,
		"url":    req.Metadata["url"],
		"source": "browser-extension",
	}

	_, err := h.docSvc.SaveRawText(c.Request.Context(), req.TextContent, title, meta, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Content uploaded"})
}

// --- Management endpoints (cookie auth, admin/manager only) ---

func (h *BrowserExtensionHandler) ApiKeys(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	keys, err := h.extSvc.ListKeys(c.Request.Context(), &user.ID, user.Role == "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "apiKeys": keys})
}

func (h *BrowserExtensionHandler) GenerateApiKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	key, err := h.extSvc.CreateKey(c.Request.Context(), &user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "apiKey": key.Key})
}

func (h *BrowserExtensionHandler) DeleteApiKey(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	if err := h.extSvc.DeleteKey(c.Request.Context(), id, &user.ID, user.Role == "admin"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterBrowserExtensionRoutes(
	r *gin.RouterGroup,
	extSvc *services.BrowserExtensionService,
	wsSvc *services.WorkspaceService,
	docSvc *services.DocumentService,
	authSvc *services.AuthService,
	cfg *config.Config,
) {
	h := NewBrowserExtensionHandler(extSvc, wsSvc, docSvc, authSvc, cfg)
	brx := middleware.ValidBrowserExtensionApiKey(extSvc, authSvc, cfg)

	// Extension-facing
	r.GET("/browser-extension/check", brx, h.Check)
	r.DELETE("/browser-extension/disconnect", brx, h.Disconnect)
	r.GET("/browser-extension/workspaces", brx, h.Workspaces)
	r.POST("/browser-extension/embed-content", brx, h.EmbedContent)
	r.POST("/browser-extension/upload-content", brx, h.UploadContent)

	// Management
	cookieAuth := middleware.ValidatedRequest(authSvc)
	roleValid := middleware.FlexUserRoleValid([]string{"admin", "manager"})
	r.GET("/browser-extension/api-keys", cookieAuth, roleValid, h.ApiKeys)
	r.POST("/browser-extension/api-keys/new", cookieAuth, roleValid, h.GenerateApiKey)
	r.DELETE("/browser-extension/api-keys/:id", cookieAuth, roleValid, h.DeleteApiKey)
}
