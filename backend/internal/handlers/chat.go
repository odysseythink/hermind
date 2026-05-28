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
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"gorm.io/gorm"
)

type ChatHandler struct {
	chatSvc *services.ChatService
}

func NewChatHandler(chatSvc *services.ChatService) *ChatHandler {
	return &ChatHandler{chatSvc: chatSvc}
}

func writeSSEChunk(w http.ResponseWriter, chunk dto.StreamChatResponse) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("data: "))
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\n\n"))
	return err
}

func (h *ChatHandler) StreamChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ws := c.MustGet("workspace").(*models.Workspace)
	user := c.MustGet("user").(*models.User)
	mlog.Info("StreamChat: workspace=", ws.Slug, " user=", user.ID)

	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		mlog.Error("StreamChat: bind json failed: ", err)
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}
	mlog.Info("StreamChat: message=", req.Message)

	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, user, nil, req)
	if err != nil {
		mlog.Error("StreamChat: chatSvc.Stream failed: ", err)
		c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	chunkCount := 0
	for chunk := range stream {
		chunkCount++
		if chunkCount <= 3 {
			txt := ""
			if chunk.TextResponse != nil {
				txt = *chunk.TextResponse
			}
			mlog.Info("StreamChat: writing chunk #", chunkCount, " type=", chunk.Type, " close=", chunk.Close, " text=", txt)
		}
		if err := writeSSEChunk(c.Writer, chunk); err != nil {
			mlog.Error("StreamChat: write chunk failed: ", err)
			break
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}
	mlog.Info("StreamChat: finished, total chunks written=", chunkCount)
}

func (h *ChatHandler) Chat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	user := c.MustGet("user").(*models.User)

	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}

	resp, err := h.chatSvc.Complete(c.Request.Context(), ws, user, nil, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ChatHandler) ApiChat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)

	var req dto.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}

	// For API chat, we don't have a user. Pass nil for user and threadID.
	resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, nil, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ChatResponse{Type: "abort", Close: true, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ChatHandler) StreamThreadChat(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ws := c.MustGet("workspace").(*models.Workspace)
	user := c.MustGet("user").(*models.User)
	thread := c.MustGet("thread").(*models.WorkspaceThread)

	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	stream, err := h.chatSvc.Stream(c.Request.Context(), ws, user, &thread.ID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.StreamChatResponse{Type: "abort", Error: utils.Ptr(err.Error()), Close: true})
		return
	}

	for chunk := range stream {
		if err := writeSSEChunk(c.Writer, chunk); err != nil {
			break
		}
		if f, ok := c.Writer.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (h *ChatHandler) SuggestedMessages(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	msgs, err := h.chatSvc.GetSuggestedMessages(c.Request.Context(), ws)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestedMessages": msgs})
}

func (h *ChatHandler) DeleteWorkspaceChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.chatSvc.DeleteWorkspaceChats(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) DeleteWorkspaceEditedChats(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	if err := h.chatSvc.DeleteWorkspaceEditedChats(c.Request.Context(), ws.ID); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) UpdateChat(c *gin.Context) {
	ws := c.MustGet("workspace").(*models.Workspace)
	var req dto.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	var body struct {
		ChatID int `json:"chatId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if body.ChatID == 0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "chatId required"})
		return
	}
	if err := h.chatSvc.UpdateChat(c.Request.Context(), ws.ID, body.ChatID, req); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ChatHandler) UpdateChatFeedback(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid chat id"})
		return
	}
	var req struct {
		FeedbackScore *bool `json:"feedbackScore"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.chatSvc.UpdateChatFeedback(c.Request.Context(), chatID, req.FeedbackScore); err != nil {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RegisterChatRoutes(r *gin.RouterGroup, chatSvc *services.ChatService, authSvc *services.AuthService, db *gorm.DB) {
	h := NewChatHandler(chatSvc)
	r.POST("/workspace/:slug/stream-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.StreamChat)
	r.POST("/workspace/:slug/chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.Chat)
	r.GET("/workspace/:slug/suggested-messages",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.SuggestedMessages)
	r.POST("/workspace/:slug/thread/:threadSlug/stream-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		middleware.ValidWorkspaceAndThreadSlug(db),
		h.StreamThreadChat)
	r.DELETE("/workspace/:slug/delete-chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteWorkspaceChats)
	r.DELETE("/workspace/:slug/delete-edited-chats",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.DeleteWorkspaceEditedChats)
	r.POST("/workspace/:slug/update-chat",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UpdateChat)
	r.POST("/workspace/:slug/chat-feedback/:chatId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		middleware.ValidWorkspaceSlug(db),
		h.UpdateChatFeedback)
}
