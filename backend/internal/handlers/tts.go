package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/internal/tts"
)

// TTSHandler exposes text-to-speech for chat responses.
type TTSHandler struct {
	chatSvc *services.ChatService
	tts     tts.Provider
}

// NewTTSHandler creates a TTSHandler.
func NewTTSHandler(chatSvc *services.ChatService, ttsProv tts.Provider) *TTSHandler {
	return &TTSHandler{chatSvc: chatSvc, tts: ttsProv}
}

// RegisterTTSRoutes wires the TTS endpoint.
func RegisterTTSRoutes(r *gin.RouterGroup, h *TTSHandler, authSvc *services.AuthService) {
	r.GET("/workspace/:slug/tts/:chatId",
		middleware.ValidatedRequest(authSvc),
		middleware.FlexUserRoleValid([]string{"all"}),
		h.Synthesize,
	)
}

// Synthesize returns synthesized audio for a chat response.
func (h *TTSHandler) Synthesize(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chatId"})
		return
	}

	chat, err := h.chatSvc.GetChatByID(c.Request.Context(), chatID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	// Response is a JSON string {"text":"...","sources":[...]} — try to extract text.
	var resp struct{ Text string `json:"text"` }
	_ = json.Unmarshal([]byte(chat.Response), &resp)
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		// Fall back to raw response if JSON unmarshal fails.
		text = strings.TrimSpace(chat.Response)
	}
	if text == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no text to synthesize"})
		return
	}

	out, err := h.tts.Synthesize(c.Request.Context(), text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, out.ContentType, out.Audio)
}
