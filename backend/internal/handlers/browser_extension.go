package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type BrowserExtensionHandler struct{}

func NewBrowserExtensionHandler() *BrowserExtensionHandler {
	return &BrowserExtensionHandler{}
}

func (h *BrowserExtensionHandler) ApiKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "apiKeys": []any{}})
}

func (h *BrowserExtensionHandler) GenerateApiKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"apiKey": "stub-key-not-implemented"})
}

func (h *BrowserExtensionHandler) DeleteApiKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterBrowserExtensionRoutes(r *gin.RouterGroup, authSvc *services.AuthService) {
	h := NewBrowserExtensionHandler()
	r.GET("/browser-extension/api-keys", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.ApiKeys)
	r.POST("/browser-extension/api-keys/new", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.GenerateApiKey)
	r.DELETE("/browser-extension/api-keys/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.DeleteApiKey)
}
