package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"gorm.io/gorm"
)

type EmbedHandler struct {
	embedSvc *services.EmbedService
}

func NewEmbedHandler(embedSvc *services.EmbedService) *EmbedHandler {
	return &EmbedHandler{embedSvc: embedSvc}
}

func (h *EmbedHandler) StreamChat(c *gin.Context) {
	embed := c.MustGet("embedConfig").(*models.EmbedConfig)
	req := c.MustGet("embedRequest").(*dto.EmbedStreamChatRequest)
	conn := c.MustGet("connection").(*dto.ConnectionMeta)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	stream, err := h.embedSvc.StreamChat(c.Request.Context(), embed, req, conn)
	if err != nil {
		msg := err.Error()
		chunk := dto.StreamChatResponse{UUID: "", Type: "abort", Close: true, Error: &msg}
		data, _ := json.Marshal(chunk)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(data)
		c.Writer.Write([]byte("\n\n"))
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}

	for chunk := range stream {
		data, _ := json.Marshal(chunk)
		c.Writer.Write([]byte("data: "))
		c.Writer.Write(data)
		c.Writer.Write([]byte("\n\n"))
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func (h *EmbedHandler) GetSessionHistory(c *gin.Context) {
	embed := c.MustGet("embedConfig").(*models.EmbedConfig)
	sessionID := c.Param("sessionId")

	chats, err := h.embedSvc.ListChats(c.Request.Context(), embed.ID, &sessionID, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}

	var history []dto.EmbedChatHistoryItem
	for i := len(chats) - 1; i >= 0; i-- {
		chat := chats[i]
		if !chat.Include {
			continue
		}
		var respObj map[string]any
		if err := json.Unmarshal([]byte(chat.Response), &respObj); err != nil {
			// Fallback to raw response if JSON parsing fails
			history = append(history, dto.EmbedChatHistoryItem{
				Role:    "user",
				Content: chat.Prompt,
				SentAt:  chat.CreatedAt,
			})
			history = append(history, dto.EmbedChatHistoryItem{
				Role:    "assistant",
				Content: chat.Response,
				SentAt:  chat.CreatedAt,
			})
			continue
		}
		text, _ := respObj["text"].(string)
		history = append(history, dto.EmbedChatHistoryItem{
			Role:    "user",
			Content: chat.Prompt,
			SentAt:  chat.CreatedAt,
		})
		history = append(history, dto.EmbedChatHistoryItem{
			Role:    "assistant",
			Content: text,
			SentAt:  chat.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.EmbedHistoryResponse{History: history})
}

func (h *EmbedHandler) DeleteSession(c *gin.Context) {
	embed := c.MustGet("embedConfig").(*models.EmbedConfig)
	sessionID := c.Param("sessionId")

	if err := h.embedSvc.MarkHistoryInvalid(c.Request.Context(), embed.ID, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *EmbedHandler) ListEmbedConfigs(c *gin.Context) {
	embeds, err := h.embedSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.EmbedConfigListResponse{Embeds: embeds})
}

func (h *EmbedHandler) CreateEmbedConfig(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	var req dto.CreateEmbedConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	embed, err := h.embedSvc.Create(c.Request.Context(), req, &user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"embed": embed, "error": nil})
}

func (h *EmbedHandler) UpdateEmbedConfig(c *gin.Context) {
	embed := c.MustGet("embedConfig").(*models.EmbedConfig)
	var req dto.UpdateEmbedConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.embedSvc.Update(c.Request.Context(), embed.ID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *EmbedHandler) DeleteEmbedConfig(c *gin.Context) {
	embed := c.MustGet("embedConfig").(*models.EmbedConfig)
	if err := h.embedSvc.Delete(c.Request.Context(), embed.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *EmbedHandler) ListAllChats(c *gin.Context) {
	var req dto.ListEmbedChatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Offset = 0
		req.Limit = 20
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	var total int64
	_ = h.embedSvc.CountAllChats(c.Request.Context(), &total)

	chats, err := h.embedSvc.ListAllChatsPaginated(c.Request.Context(), req.Limit, req.Offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.EmbedChatListResponse{
		Chats:      chats,
		HasPages:   total > int64(req.Offset+req.Limit),
		TotalChats: total,
	})
}

func (h *EmbedHandler) DeleteChat(c *gin.Context) {
	chatIDStr := c.Param("chatId")
	chatID, err := strconv.Atoi(chatIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid chat ID"})
		return
	}
	if err := h.embedSvc.DeleteChat(c.Request.Context(), chatID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterEmbedRoutes(r *gin.RouterGroup, svc *services.EmbedService, db *gorm.DB) {
	h := NewEmbedHandler(svc)
	r.POST("/embed/:embedId/stream-chat",
		middleware.ValidEmbedConfig(db),
		middleware.SetConnectionMeta(),
		middleware.CanRespond(db, svc),
		h.StreamChat)
	r.GET("/embed/:embedId/:sessionId",
		middleware.ValidEmbedConfig(db),
		h.GetSessionHistory)
	r.DELETE("/embed/:embedId/:sessionId",
		middleware.ValidEmbedConfig(db),
		h.DeleteSession)
}

func RegisterEmbedManagementRoutes(r *gin.RouterGroup, svc *services.EmbedService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewEmbedHandler(svc)
	r.GET("/embeds",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListEmbedConfigs)
	r.POST("/embeds/new",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.CreateEmbedConfig)
	r.POST("/embed/update/:embedId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		middleware.ValidEmbedConfigId(db),
		h.UpdateEmbedConfig)
	r.DELETE("/embed/:embedId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		middleware.ValidEmbedConfigId(db),
		h.DeleteEmbedConfig)
	r.POST("/embed/chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.ListAllChats)
	r.DELETE("/embed/chats/:chatId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"admin"}),
		h.DeleteChat)
}
