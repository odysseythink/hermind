package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
)

type APISystemHandler struct {
	sysSvc     *services.SystemService
	vectorSvc  *services.VectorService
	docSvc     *services.DocumentService
	wsChatSvc  *services.WorkspaceChatService
	webSysHdlr *SystemHandler
}

func NewAPISystemHandler(sysSvc *services.SystemService, vectorSvc *services.VectorService, docSvc *services.DocumentService, wsChatSvc *services.WorkspaceChatService, webSysHdlr *SystemHandler) *APISystemHandler {
	return &APISystemHandler{sysSvc: sysSvc, vectorSvc: vectorSvc, docSvc: docSvc, wsChatSvc: wsChatSvc, webSysHdlr: webSysHdlr}
}

func (h *APISystemHandler) GetAll(c *gin.Context) {
	settings, err := h.sysSvc.GetAllSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *APISystemHandler) EnvDump(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (h *APISystemHandler) VectorCount(c *gin.Context) {
	n, err := h.vectorSvc.TotalVectors(c.Request.Context())
	if err != nil {
		// Nil provider is not an error for API v1 — return 0 (Node parity).
		c.JSON(http.StatusOK, gin.H{"vectorCount": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{"vectorCount": n})
}

func (h *APISystemHandler) UpdateEnv(c *gin.Context) {
	h.webSysHdlr.UpdateEnv(c)
}

func (h *APISystemHandler) ExportChats(c *gin.Context) {
	format := c.DefaultQuery("type", "jsonl")
	contentType, data, err := h.wsChatSvc.ExportChats(c.Request.Context(), format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, data)
}

func (h *APISystemHandler) RemoveDocuments(c *gin.Context) {
	var req dto.APISystemRemoveDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	for _, name := range req.Names {
		if err := h.docSvc.PurgeByDocName(c.Request.Context(), name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Documents removed successfully"})
}

func RegisterAPISystemRoutes(r *gin.RouterGroup, apiKeySvc *services.APIKeyService, sysSvc *services.SystemService, vectorSvc *services.VectorService, docSvc *services.DocumentService, wsChatSvc *services.WorkspaceChatService, webSysHdlr *SystemHandler) {
	h := NewAPISystemHandler(sysSvc, vectorSvc, docSvc, wsChatSvc, webSysHdlr)
	r.GET("/v1/system", middleware.ValidAPIKey(apiKeySvc), h.GetAll)
	r.GET("/v1/system/env-dump", middleware.ValidAPIKey(apiKeySvc), h.EnvDump)
	r.GET("/v1/system/vector-count", middleware.ValidAPIKey(apiKeySvc), h.VectorCount)
	r.POST("/v1/system/update-env", middleware.ValidAPIKey(apiKeySvc), h.UpdateEnv)
	r.GET("/v1/system/export-chats", middleware.ValidAPIKey(apiKeySvc), h.ExportChats)
	r.DELETE("/v1/system/remove-documents", middleware.ValidAPIKey(apiKeySvc), h.RemoveDocuments)
}
