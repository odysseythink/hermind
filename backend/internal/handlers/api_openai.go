package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/middleware"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

type APIOpenAIHandler struct {
	wsSvc     *services.WorkspaceService
	chatSvc   *services.ChatService
	threadSvc *services.ThreadService
	emb       embedder.Embedder
	db        *gorm.DB
	cfg       *config.Config
}

func NewAPIOpenAIHandler(
	wsSvc *services.WorkspaceService,
	chatSvc *services.ChatService,
	threadSvc *services.ThreadService,
	emb embedder.Embedder,
	db *gorm.DB,
	cfg *config.Config,
) *APIOpenAIHandler {
	return &APIOpenAIHandler{
		wsSvc: wsSvc, chatSvc: chatSvc, threadSvc: threadSvc,
		emb: emb, db: db, cfg: cfg,
	}
}

func (h *APIOpenAIHandler) Models(c *gin.Context) {
	workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	data := make([]gin.H, 0, len(workspaces))
	for _, ws := range workspaces {
		provider := h.cfg.LLMProvider
		if ws.ChatProvider != nil && *ws.ChatProvider != "" {
			provider = *ws.ChatProvider
		}
		model := h.cfg.LLMModel
		if ws.ChatModel != nil && *ws.ChatModel != "" {
			model = *ws.ChatModel
		}
		data = append(data, gin.H{
			"id":       ws.Slug,
			"object":   "model",
			"created":  ws.CreatedAt.Unix(),
			"owned_by": provider + "-" + model,
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func (h *APIOpenAIHandler) VectorStores(c *gin.Context) {
	workspaces, err := h.wsSvc.List(c.Request.Context(), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Pagination: OpenAI uses cursor-based pagination with "limit" and "after".
	limit := 20
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	after := c.Query("after")

	// Slice according to cursor.
	startIdx := 0
	if after != "" {
		found := false
		for i, ws := range workspaces {
			if ws.Slug == after {
				startIdx = i + 1
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusOK, gin.H{
				"first_id": "",
				"last_id":  "",
				"data":     []any{},
				"has_more": false,
			})
			return
		}
	}
	endIdx := startIdx + limit
	hasMore := false
	if endIdx < len(workspaces) {
		hasMore = true
	} else {
		endIdx = len(workspaces)
	}
	page := workspaces[startIdx:endIdx]

	provider := h.cfg.VectorDB
	if provider == "" {
		provider = "lancedb"
	}
	data := make([]gin.H, 0, len(page))
	for _, ws := range page {
		var total int64
		h.db.Model(&models.WorkspaceDocument{}).
			Where("workspace_id = ?", ws.ID).Count(&total)
		data = append(data, gin.H{
			"id":          ws.Slug,
			"object":      "vector_store",
			"name":        ws.Name,
			"file_counts": gin.H{"total": total},
			"provider":    provider,
		})
	}
	var firstID, lastID string
	if len(data) > 0 {
		firstID = page[0].Slug
		lastID = page[len(page)-1].Slug
	}
	c.JSON(http.StatusOK, gin.H{
		"first_id": firstID,
		"last_id":  lastID,
		"data":     data,
		"has_more": hasMore,
	})
}

func (h *APIOpenAIHandler) Embeddings(c *gin.Context) {
	if h.emb == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "embedder not configured"})
		return
	}
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Accept either "input" or legacy "inputs".
	rawInput := raw["input"]
	if rawInput == nil {
		rawInput = raw["inputs"]
	}
	var texts []string
	switch v := rawInput.(type) {
	case string:
		texts = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				texts = append(texts, s)
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "All inputs to be embedded must be strings."})
			return
		}
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Input must be a string or array of strings."})
		return
	}
	if len(texts) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Input array cannot be empty."})
		return
	}
	vecs, err := h.emb.EmbedTexts(c.Request.Context(), texts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	data := make([]gin.H, len(vecs))
	for i, v := range vecs {
		data[i] = gin.H{"object": "embedding", "embedding": v, "index": i}
	}
	model := h.cfg.EmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data, "model": model})
}

// extractMessageParts parses an OpenAI message Content (string or []{type,text|image_url})
// and returns the plain text plus any image URLs found.
func extractMessageParts(content any) (text string, imageURLs []string) {
	if s, ok := content.(string); ok {
		return s, nil
	}
	parts, ok := content.([]any)
	if !ok {
		return "", nil
	}
	var sb strings.Builder
	first := true
	for _, p := range parts {
		m, ok := p.(map[string]any)
		if !ok {
			continue
		}
		switch t, _ := m["type"].(string); t {
		case "text":
			if !first {
				sb.WriteString("\n")
			}
			txt, _ := m["text"].(string)
			sb.WriteString(txt)
			first = false
		case "image_url":
			if iu, ok := m["image_url"].(map[string]any); ok {
				if url, ok := iu["url"].(string); ok && url != "" {
					imageURLs = append(imageURLs, url)
				}
			}
		}
	}
	return sb.String(), imageURLs
}

// openaiHistoryToCoreMessages converts the conversation prefix (without the
// trailing user message and any role:"system" entries) into core.Message slice.
// Role "user" → MESSAGE_ROLE_USER, "assistant" → MESSAGE_ROLE_ASSISTANT.
// Supports multimodal content (text + image_url).
func openaiHistoryToCoreMessages(msgs []dto.OpenAIMessage) []core.Message {
	out := make([]core.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		var role core.MessageRoleType
		switch m.Role {
		case "user":
			role = core.MESSAGE_ROLE_USER
		case "assistant":
			role = core.MESSAGE_ROLE_ASSISTANT
		default:
			continue
		}
		text, images := extractMessageParts(m.Content)
		content := core.NewTextContent(text)
		for _, url := range images {
			content = append(content, core.ImagePart{URL: url})
		}
		out = append(out, core.Message{
			Role:    role,
			Content: content,
		})
	}
	return out
}

func (h *APIOpenAIHandler) ChatCompletions(c *gin.Context) {
	var req dto.OpenAIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages cannot be empty"})
		return
	}
	// Last message must be role:user (Node parity).
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" {
		c.JSON(http.StatusBadRequest, gin.H{
			"id":           uuid.New().String(),
			"type":         "abort",
			"textResponse": nil,
			"sources":      []any{},
			"close":        true,
			"error":        "No user prompt found. Must be last element in message array with 'user' role.",
		})
		return
	}
	prompt, imageURLs := extractMessageParts(last.Content)

	// Resolve workspace (and optional thread via slug:threadSlug).
	// SplitN(..., 2) so threadSlug may itself contain colons.
	wsSlug, threadSlug := req.Model, ""
	if parts := strings.SplitN(req.Model, ":", 2); len(parts) == 2 {
		wsSlug, threadSlug = parts[0], parts[1]
	}
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), wsSlug)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized) // Node parity
		return
	}
	var threadID *int
	if threadSlug != "" {
		thread, err := h.threadSvc.GetBySlug(c.Request.Context(), ws.ID, threadSlug)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		threadID = &thread.ID
	}

	// Extract system message + history override.
	var systemOverride *string
	var historyMsgs []dto.OpenAIMessage
	for _, m := range req.Messages[:len(req.Messages)-1] {
		if m.Role == "system" {
			s, _ := extractMessageParts(m.Content)
			if s != "" {
				systemOverride = &s
			}
			continue
		}
		historyMsgs = append(historyMsgs, m)
	}
	history := openaiHistoryToCoreMessages(historyMsgs)

	chatReq := dto.ChatRequest{
		Message:              prompt,
		Attachments:          imageURLs,
		SystemPromptOverride: systemOverride,
		TemperatureOverride:  req.Temperature,
		HistoryOverride:      history,
	}

	if !req.Stream {
		resp, err := h.chatSvc.Complete(c.Request.Context(), ws, nil, threadID, chatReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		promptTokens := countPromptTokens(systemOverride, history, prompt)
		completionTokens := utils.EstimateTokenCount(resp.TextResponse)
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl-" + uuid.New().String(),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []gin.H{{
				"index":         0,
				"message":       gin.H{"role": "assistant", "content": resp.TextResponse},
				"finish_reason": "stop",
			}},
			"usage": gin.H{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      promptTokens + completionTokens,
			},
		})
		return
	}
	// Streaming path — pass prompt token count so it can emit usage.
	h.chatCompletionsStream(c, ws, threadID, chatReq, req.Model, countPromptTokens(systemOverride, history, prompt))
}

func (h *APIOpenAIHandler) chatCompletionsStream(c *gin.Context, ws *models.Workspace, threadID *int, chatReq dto.ChatRequest, modelStr string, promptTokens int) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	streamReq := dto.StreamChatRequest{
		Message:              chatReq.Message,
		SystemPromptOverride: chatReq.SystemPromptOverride,
		TemperatureOverride:  chatReq.TemperatureOverride,
		HistoryOverride:      chatReq.HistoryOverride,
	}
	chunks, err := h.chatSvc.Stream(c.Request.Context(), ws, nil, threadID, streamReq)
	if err != nil {
		writeOpenAIErrorFrame(c.Writer, flusher, err.Error())
		return
	}

	chunkID := "chatcmpl-" + uuid.New().String()
	created := time.Now().Unix()
	var completionTokens int

	// SSE keepalive — write a comment-line ping every 15s so proxies don't drop idle connections.
	// All writes to c.Writer are protected by writeMu because http.ResponseWriter is not concurrent-safe.
	var writeMu sync.Mutex
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				fmt.Fprint(c.Writer, ": ping\n\n")
				flusher.Flush()
				writeMu.Unlock()
			case <-done:
				return
			}
		}
	}()

	for chunk := range chunks {
		switch chunk.Type {
		case "textResponseChunk":
			delta := ""
			if chunk.TextResponse != nil {
				delta = *chunk.TextResponse
			}
			completionTokens += utils.EstimateTokenCount(delta)
			writeMu.Lock()
			writeOpenAIDeltaFrame(c.Writer, flusher, chunkID, created, modelStr,
				gin.H{"content": delta}, nil)
			writeMu.Unlock()
		case "finalizeResponseStream":
			writeMu.Lock()
			writeOpenAIDeltaFrame(c.Writer, flusher, chunkID, created, modelStr,
				gin.H{}, openaiStringPtr("stop"))
			writeMu.Unlock()
		case "abort":
			errStr := "stream aborted"
			if chunk.Error != nil {
				errStr = *chunk.Error
			}
			close(done)
			writeMu.Lock()
			writeOpenAIErrorFrame(c.Writer, flusher, errStr)
			writeMu.Unlock()
			return
		}
		if chunk.Close {
			break
		}
	}

	close(done)
	// Emit usage chunk before [DONE] (OpenAI-compat final chunk).
	writeMu.Lock()
	if promptTokens > 0 || completionTokens > 0 {
		usagePayload := gin.H{
			"id":      chunkID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelStr,
			"choices": []gin.H{},
			"usage": gin.H{
				"prompt_tokens":     promptTokens,
				"completion_tokens": completionTokens,
				"total_tokens":      promptTokens + completionTokens,
			},
		}
		raw, _ := json.Marshal(usagePayload)
		fmt.Fprintf(c.Writer, "data: %s\n\n", raw)
		flusher.Flush()
	}
	fmt.Fprint(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
	writeMu.Unlock()
}

// countPromptTokens estimates the token count of everything sent to the LLM as context.
func countPromptTokens(systemOverride *string, history []core.Message, prompt string) int {
	total := 0
	if systemOverride != nil {
		total += utils.EstimateTokenCount(*systemOverride)
	}
	for _, m := range history {
		total += utils.EstimateTokenCount(string(m.Role))
		total += utils.EstimateTokenCount(m.Text())
	}
	total += utils.EstimateTokenCount(prompt)
	return total
}

func writeOpenAIDeltaFrame(w gin.ResponseWriter, flusher http.Flusher, id string, created int64, model string, delta gin.H, finishReason *string) {
	payload := gin.H{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": []gin.H{{
			"index":         0,
			"delta":         delta,
			"finish_reason": finishReason,
		}},
	}
	raw, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", raw)
	flusher.Flush()
}

func writeOpenAIErrorFrame(w gin.ResponseWriter, flusher http.Flusher, msg string) {
	payload := gin.H{
		"error": gin.H{"message": msg, "type": "api_error"},
	}
	raw, _ := json.Marshal(payload)
	fmt.Fprintf(w, "data: %s\n\n", raw)
	flusher.Flush()
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func openaiStringPtr(s string) *string { return &s }

func RegisterAPIOpenAIRoutes(
	r *gin.RouterGroup,
	apiKeySvc *services.APIKeyService,
	wsSvc *services.WorkspaceService,
	chatSvc *services.ChatService,
	threadSvc *services.ThreadService,
	emb embedder.Embedder,
	db *gorm.DB,
	cfg *config.Config,
) {
	h := NewAPIOpenAIHandler(wsSvc, chatSvc, threadSvc, emb, db, cfg)
	r.GET("/v1/openai/models", middleware.ValidAPIKey(apiKeySvc), h.Models)
	r.GET("/v1/openai/vector_stores", middleware.ValidAPIKey(apiKeySvc), h.VectorStores)
	r.POST("/v1/openai/embeddings", middleware.ValidAPIKey(apiKeySvc), h.Embeddings)
	r.POST("/v1/openai/chat/completions", middleware.ValidAPIKey(apiKeySvc), h.ChatCompletions)
}
