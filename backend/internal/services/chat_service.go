package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	agentcompression "github.com/odysseythink/hermind/backend/internal/agent/compression"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

var (
	ErrNothingToCompress       = errors.New("nothing to compress")
	ErrCompressionNotAvailable = errors.New("compression not available")
)

// CompactionResult is the outcome of a manual or automatic compression.
type CompactionResult struct {
	Before       int     `json:"before"`
	After        int     `json:"after"`
	SavedPct     float64 `json:"savedPct"`
	Summary      string  `json:"summary"`
	FallbackUsed bool    `json:"fallbackUsed"`
}

type ChatService struct {
	db           *gorm.DB
	cfg          *config.Config
	vectorSvc    *VectorService
	llmProv      providers.LLMProvider
	embedder     embedder.Embedder
	agentInvoker AgentInvoker
	reranker     reranker.Reranker
	memInj       *MemoryInjector
	autoTitleSvc *AutoTitleService
	compStore    *agentcompression.CompactionStore
	sysSvc       *SystemService
}

func NewChatService(db *gorm.DB, cfg *config.Config, vectorSvc *VectorService, llmProv providers.LLMProvider, embedder embedder.Embedder, agentInvoker AgentInvoker, reranker reranker.Reranker, memInj *MemoryInjector, autoTitleSvc *AutoTitleService, compStore *agentcompression.CompactionStore, sysSvc *SystemService) *ChatService {
	return &ChatService{db: db, cfg: cfg, vectorSvc: vectorSvc, llmProv: llmProv, embedder: embedder, agentInvoker: agentInvoker, reranker: reranker, memInj: memInj, autoTitleSvc: autoTitleSvc, compStore: compStore, sysSvc: sysSvc}
}

func (s *ChatService) buildRAGContext(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, message string, systemPromptOverride *string, historyOverride []core.Message) (systemPrompt string, sources []any, history []core.Message, err error) {
	if historyOverride != nil {
		history = historyOverride
	} else {
		historyLimit := ws.OpenAiHistory
		if historyLimit <= 0 {
			historyLimit = 20
		}
		var afterChatID int
		var summary string
		if s.compStore != nil {
			summary, afterChatID, _ = s.compStore.SeedForSession(ws.ID, threadID)
		}
		history, _, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit, afterChatID)
		if err != nil {
			return "", nil, nil, err
		}
		if summary != "" {
			history = append([]core.Message{core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, summary)}, history...)
		}
	}

	// PR2: API v1 OpenAI-compat may pass an explicit override; treat empty string as "no override".
	if systemPromptOverride != nil && *systemPromptOverride != "" {
		systemPrompt = *systemPromptOverride
	} else if ws.OpenAiPrompt != nil {
		systemPrompt = *ws.OpenAiPrompt
	}

	// Inject long-term memories (no-op when memInj is nil or disabled).
	var userID *int
	if user != nil {
		userID = &user.ID
	}
	systemPrompt = s.memInj.PromptWithMemories(ctx, systemPrompt, userID, ws.ID, message, history)

	if s.vectorSvc.provider != nil {
		topN := 4
		if ws.TopN != nil {
			topN = *ws.TopN
		}
		threshold := 0.25
		if ws.SimilarityThreshold != nil {
			threshold = *ws.SimilarityThreshold
		}

		var queryVector []float32
		if s.embedder != nil {
			qv, err := s.embedder.EmbedQuery(ctx, message)
			if err == nil {
				queryVector = qv
			} else {
				mlog.Error("embed query failed: ", err)
			}
		}

		results, err := s.vectorSvc.SimilaritySearch(ctx, ws.Slug, queryVector, vectordb.SearchOptions{
			TopN:                topN,
			SimilarityThreshold: threshold,
		})
		if err == nil {
			if s.reranker != nil {
				texts := make([]string, len(results))
				for i, r := range results {
					texts[i] = r.Text
				}
				if ranked, err := s.reranker.Rerank(ctx, message, texts, topN); err == nil {
					reordered := make([]vectordb.SearchResult, 0, len(ranked))
					for _, rr := range ranked {
						if rr.Index >= 0 && rr.Index < len(results) {
							reordered = append(reordered, results[rr.Index])
						}
					}
					results = reordered
				} else {
					mlog.Warning("rerank failed, using raw search results", mlog.Err(err))
				}
			}
			var ragTexts []string
			for _, r := range results {
				sources = append(sources, map[string]any{
					"docId":    r.DocId,
					"text":     r.Text,
					"score":    r.Score,
					"metadata": r.Metadata,
				})
				ragTexts = append(ragTexts, r.Text)
			}
			if len(ragTexts) > 0 {
				systemPrompt += "\n\nContext:\n" + strings.Join(ragTexts, "\n---\n")
			}
		}
	}

	return systemPrompt, sources, history, nil
}

func (s *ChatService) Stream(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.StreamChatRequest) (<-chan dto.StreamChatResponse, error) {
	msgID := uuid.New().String()
	out := make(chan dto.StreamChatResponse, 16)
	mlog.Info("ChatService.Stream: start msgID=", msgID, " workspace=", ws.Slug, " message=", req.Message)

	go func() {
		defer close(out)

		// PR-AR-4: @agent handoff to WebSocket runtime
		if s.agentInvoker != nil {
			invoked, err := s.agentInvoker.IsAgentInvocation(ctx, ws, req.Message)
			if err != nil {
				mlog.Warning("ChatService.Stream: IsAgentInvocation error: ", err)
				// fall through to non-agent path
			} else if invoked {
				var thread *models.WorkspaceThread
				if threadID != nil {
					thread = &models.WorkspaceThread{ID: *threadID}
				}
				ho, err := s.agentInvoker.PrepareInvocationHandoff(ctx, ws, user, thread, req.Message)
				if err != nil {
					mlog.Error("ChatService.Stream: PrepareInvocationHandoff failed: ", err)
					out <- dto.StreamChatResponse{
						UUID: msgID, Type: "abort", Close: true,
						Error: utils.Ptr("agent invocation could not be prepared: " + err.Error()),
					}
					return
				}
				out <- dto.StreamChatResponse{
					UUID:           msgID,
					Type:           "agentInitWebsocketConnection",
					WebsocketUUID:  &ho.UUID,
					WebsocketToken: &ho.WSToken,
					Close:          false,
				}
				out <- dto.StreamChatResponse{
					UUID:         msgID,
					Type:         "statusResponse",
					TextResponse: utils.Ptr("@agent: Swapping over to agent chat. Type /exit to exit agent execution loop early."),
					Close:        true,
					Animate:      true,
				}
				return // do NOT run RAG / LLM stream
			}
		}

		var fullText strings.Builder

		systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message, req.SystemPromptOverride, req.HistoryOverride)
		if err != nil {
			mlog.Error("ChatService.Stream: build RAG context failed: ", err)
			out <- dto.StreamChatResponse{
				UUID: msgID, Type: "abort",
				Close: true, Error: utils.Ptr(err.Error()),
			}
			return
		}
		mlog.Info("ChatService.Stream: built history with ", len(history), " messages")

		// Compression gate
		history, err = s.tryCompressHistory(ctx, ws, threadID, history)
		if err != nil {
			mlog.Warning("ChatService.Stream: compression error: ", err)
		}

		// Add current user message to history
		userContent := core.NewTextContent(req.Message)
		for _, url := range req.Attachments {
			userContent = append(userContent, core.ImagePart{URL: url})
		}
		messages := append(history, core.Message{
			Role:    core.MESSAGE_ROLE_USER,
			Content: userContent,
		})

		// Stream via Pantheon LLM
		mlog.Info("ChatService.Stream: calling llmProv.Stream")
		chunks, err := s.llmProv.Stream(ctx, messages, systemPrompt, req.TemperatureOverride)
		if err != nil {
			mlog.Error("ChatService.Stream: llm stream init failed: ", err)
			out <- dto.StreamChatResponse{
				UUID: msgID, Type: "abort",
				Close: true, Error: utils.Ptr(err.Error()),
			}
			return
		}
		mlog.Info("ChatService.Stream: llmProv.Stream returned channel")

		chunkCount := 0
		for chunk := range chunks {
			select {
			case <-ctx.Done():
				mlog.Info("ChatService.Stream: context done during chunk loop")
				return
			default:
			}
			chunkCount++
			if chunk.Err != nil {
				mlog.Error("ChatService.Stream: chunk error: ", chunk.Err)
				out <- dto.StreamChatResponse{
					UUID: msgID, Type: "abort",
					Close: true, Error: utils.Ptr(chunk.Err.Error()),
				}
				return
			}
			if chunkCount <= 3 {
				mlog.Info("ChatService.Stream: received chunk #", chunkCount, " delta=", chunk.TextDelta, " finish=", chunk.FinishReason)
			}
			if chunk.TextDelta != "" {
				fullText.WriteString(chunk.TextDelta)
				out <- dto.StreamChatResponse{
					UUID:         msgID,
					Type:         "textResponseChunk",
					TextResponse: utils.Ptr(chunk.TextDelta),
					Sources:      sources,
				}
			}
			if chunk.FinishReason != "" {
				mlog.Info("ChatService.Stream: finish reason received, total chunks=", chunkCount)
				out <- dto.StreamChatResponse{
					UUID:         msgID,
					Type:         "finalizeResponseStream",
					TextResponse: utils.Ptr(""),
					Close:        true,
					Sources:      sources,
				}
				s.saveChatResponse(ctx, ws, user, threadID, req.Message, fullText.String())
				return
			}
		}

		// If stream ended without finish reason, close gracefully
		mlog.Info("ChatService.Stream: channel closed without finish reason, total chunks=", chunkCount)
		out <- dto.StreamChatResponse{
			UUID:         msgID,
			Type:         "finalizeResponseStream",
			TextResponse: utils.Ptr(""),
			Close:        true,
			Sources:      sources,
		}
		s.saveChatResponse(ctx, ws, user, threadID, req.Message, fullText.String())
	}()

	return out, nil
}

func (s *ChatService) Complete(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, req dto.ChatRequest) (*dto.ChatResponse, error) {
	msgID := uuid.New().String()

	if strings.TrimSpace(req.Message) == "" {
		return &dto.ChatResponse{ID: msgID, Type: "abort", Close: true, Error: "Message is empty."}, nil
	}

	systemPrompt, sources, history, err := s.buildRAGContext(ctx, ws, user, threadID, req.Message, req.SystemPromptOverride, req.HistoryOverride)
	if err != nil {
		return &dto.ChatResponse{ID: msgID, Type: "abort", Close: true, Error: err.Error()}, nil
	}

	// Compression gate
	history, err = s.tryCompressHistory(ctx, ws, threadID, history)
	if err != nil {
		mlog.Warning("ChatService.Complete: compression error: ", err)
	}

	userContent := core.NewTextContent(req.Message)
	for _, url := range req.Attachments {
		userContent = append(userContent, core.ImagePart{URL: url})
	}
	messages := append(history, core.Message{
		Role:    core.MESSAGE_ROLE_USER,
		Content: userContent,
	})

	text, err := s.llmProv.Complete(ctx, messages, systemPrompt, req.TemperatureOverride)
	if err != nil {
		return &dto.ChatResponse{ID: msgID, Type: "abort", Close: true, Error: err.Error()}, nil
	}

	s.saveChatResponse(ctx, ws, user, threadID, req.Message, text)

	return &dto.ChatResponse{
		ID:           msgID,
		Type:         "textResponse",
		TextResponse: text,
		Sources:      sources,
		Close:        true,
	}, nil
}

func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID int, threadID *int, limit int, afterChatID int) ([]core.Message, int, error) {
	var chats []models.WorkspaceChat
	query := s.db.Where("workspace_id = ? AND include = ?", workspaceID, true)
	if threadID != nil {
		query = query.Where("thread_id = ?", *threadID)
	} else {
		query = query.Where("thread_id IS NULL")
	}
	if afterChatID > 0 {
		query = query.Where("id > ?", afterChatID)
	}
	if err := query.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil {
		return nil, 0, err
	}

	history := make([]core.Message, 0, len(chats)*2)
	for i := len(chats) - 1; i >= 0; i-- {
		c := chats[i]
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_USER, c.Prompt))
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, c.Response))
	}

	maxChatID := 0
	if len(chats) > 0 {
		maxChatID = chats[0].ID // DESC order: first element has highest ID
	}
	return history, maxChatID, nil
}

func (s *ChatService) saveChatResponse(ctx context.Context, ws *models.Workspace, user *models.User, threadID *int, prompt, response string) {
	respObj := map[string]any{
		"text":    response,
		"type":    "chart",
		"sources": []any{},
	}
	respJSON, _ := json.Marshal(respObj)
	chat := models.WorkspaceChat{
		WorkspaceID:   ws.ID,
		ThreadID:      threadID,
		Prompt:        prompt,
		Response:      string(respJSON),
		Include:       true,
		CreatedAt:     time.Now(),
		LastUpdatedAt: time.Now(),
	}
	if user != nil {
		chat.UserID = &user.ID
	}
	if err := s.db.Create(&chat).Error; err != nil {
		mlog.Error("save chat failed: ", err)
		return
	}
	if err := s.db.Exec("INSERT INTO workspace_chat_fts(rowid, prompt, response) VALUES (?, ?, ?)", chat.ID, chat.Prompt, chat.Response).Error; err != nil {
		mlog.Error("save chat fts5 failed: ", err)
	}
	// Trigger async auto-title generation for new threads on their first exchange.
	if threadID != nil && s.autoTitleSvc != nil {
		s.autoTitleSvc.MaybeGenerate(ctx, *threadID, prompt, response)
	}
}

// tryCompressHistory attempts to compress the conversation history before
// sending it to the LLM. If compression is disabled, the aux model is nil,
// or the threshold is not exceeded, the original history is returned unchanged.
func (s *ChatService) tryCompressHistory(ctx context.Context, ws *models.Workspace, threadID *int, history []core.Message) ([]core.Message, error) {
	if s.compStore == nil || s.sysSvc == nil {
		return history, nil
	}
	globalEnabledStr, _ := s.sysSvc.GetSetting(ctx, "context_compress_enabled")
	globalEnabled := globalEnabledStr == "true"
	if !agentcompression.IsEnabledForWorkspace(globalEnabled, ws.CompressEnabled) {
		return history, nil
	}

	aux := s.llmProv.LanguageModel()
	if aux == nil {
		return history, nil
	}

	cfg := compression.CompressionConfig{
		Enabled:             true,
		Threshold:           0.75,
		TargetRatio:         0.2,
		ProtectLast:         20,
		MaxPasses:           3,
		PerMessageMaxTokens: 8000,
	}
	if ws.CompressThreshold != nil {
		cfg.Threshold = *ws.CompressThreshold
	}
	model := ""
	if ws.ChatModel != nil {
		model = *ws.ChatModel
	}
	ctxLen := agentcompression.ContextLengthFor(model)
	if ws.CompressContextLen != nil {
		ctxLen = *ws.CompressContextLen
	}

	comp := compression.NewCompressor(cfg, aux, ctxLen)
	compressed, err := comp.Compress(ctx, history)
	if err != nil {
		mlog.Warning("ChatService: compression failed: ", err)
		return history, nil
	}
	summary := extractSummaryFromCompressed(compressed)
	if summary != "" {
		if err := s.saveCompactionAndSoftDelete(ctx, ws.ID, threadID, summary, 0, 0, false); err != nil {
			mlog.Warning("ChatService: compaction persistence failed: ", err)
		}
	}
	return compressed, nil
}

// saveCompactionAndSoftDelete persists the summary and soft-deletes the
// compressed chat rows so they are not re-read in future turns.
func (s *ChatService) saveCompactionAndSoftDelete(ctx context.Context, workspaceID int, threadID *int, summary string, beforeTokens, afterTokens int, fallbackUsed bool) error {
	maxChatID, err := s.maxChatIDForThread(ctx, workspaceID, threadID)
	if err != nil || maxChatID == 0 {
		return err
	}
	if err := s.compStore.Save(&models.ThreadCompaction{
		WorkspaceID:  workspaceID,
		ThreadID:     threadID,
		Summary:      summary,
		UpToChatID:   maxChatID,
		BeforeTokens: beforeTokens,
		AfterTokens:  afterTokens,
		FallbackUsed: fallbackUsed,
	}); err != nil {
		return fmt.Errorf("save compaction: %w", err)
	}
	if err := s.softDeleteChatsUpTo(ctx, workspaceID, threadID, maxChatID); err != nil {
		return fmt.Errorf("soft-delete chats: %w", err)
	}
	mlog.Info("chat compaction: ", beforeTokens, "→", afterTokens, " tokens")
	return nil
}

func (s *ChatService) maxChatIDForThread(ctx context.Context, workspaceID int, threadID *int) (int, error) {
	var maxID int
	q := s.db.Model(&models.WorkspaceChat{}).Select("COALESCE(MAX(id), 0)").
		Where("workspace_id = ? AND include = ?", workspaceID, true)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	if err := q.Scan(&maxID).Error; err != nil {
		return 0, err
	}
	return maxID, nil
}

func (s *ChatService) softDeleteChatsUpTo(ctx context.Context, workspaceID int, threadID *int, upToChatID int) error {
	q := s.db.Model(&models.WorkspaceChat{}).
		Where("workspace_id = ? AND id <= ?", workspaceID, upToChatID)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	return q.Update("include", false).Error
}

func extractSummaryFromCompressed(msgs []core.Message) string {
	const prefix = "[Compressed summary of earlier conversation]\n"
	for _, m := range msgs {
		if m.Role != core.MESSAGE_ROLE_ASSISTANT {
			continue
		}
		text := m.Text()
		if idx := strings.Index(text, prefix); idx >= 0 {
			return text[idx+len(prefix):]
		}
	}
	return ""
}

func (s *ChatService) GetChatByID(ctx context.Context, id int) (*models.WorkspaceChat, error) {
	var chat models.WorkspaceChat
	if err := s.db.WithContext(ctx).First(&chat, id).Error; err != nil {
		return nil, err
	}
	return &chat, nil
}

// CompressNow performs an on-demand compression of the full chat history.
// It bypasses the automatic threshold gate (always attempts compression).
// Returns ErrNothingToCompress if history is too short, or ErrCompressionNotAvailable
// if compression is disabled for the workspace.
func (s *ChatService) CompressNow(ctx context.Context, ws *models.Workspace, threadID *int, topic string) (CompactionResult, error) {
	if s.compStore == nil || s.sysSvc == nil {
		return CompactionResult{}, ErrCompressionNotAvailable
	}
	globalEnabledStr, _ := s.sysSvc.GetSetting(ctx, "context_compress_enabled")
	globalEnabled := globalEnabledStr == "true"
	if !agentcompression.IsEnabledForWorkspace(globalEnabled, ws.CompressEnabled) {
		return CompactionResult{}, ErrCompressionNotAvailable
	}

	// Read full history (no limit truncation)
	history, maxChatID, err := s.buildChatHistory(ctx, ws.ID, threadID, 999999, 0)
	if err != nil {
		return CompactionResult{}, err
	}

	const minMessagesForCompress = 4 // at least 2 user/assistant pairs
	if len(history) < minMessagesForCompress {
		return CompactionResult{}, ErrNothingToCompress
	}

	aux := s.llmProv.LanguageModel()
	if aux == nil {
		return CompactionResult{}, ErrCompressionNotAvailable
	}

	cfg := compression.CompressionConfig{
		Enabled:             true,
		Threshold:           0.75,
		TargetRatio:         0.2,
		ProtectLast:         20,
		MaxPasses:           3,
		PerMessageMaxTokens: 8000,
	}
	if ws.CompressThreshold != nil {
		cfg.Threshold = *ws.CompressThreshold
	}
	model := ""
	if ws.ChatModel != nil {
		model = *ws.ChatModel
	}
	ctxLen := agentcompression.ContextLengthFor(model)
	if ws.CompressContextLen != nil {
		ctxLen = *ws.CompressContextLen
	}
	comp := compression.NewCompressor(cfg, aux, ctxLen)

	before := estimateTokens(history)
	compressed, err := comp.Compress(ctx, history)
	if err != nil {
		return CompactionResult{}, err
	}
	after := estimateTokens(compressed)
	summary := extractSummaryFromCompressed(compressed)

	// Persist and soft-delete
	if summary != "" && maxChatID > 0 {
		if err := s.saveCompactionAndSoftDelete(ctx, ws.ID, threadID, summary, before, after, false); err != nil {
			mlog.Warning("CompressNow: persistence failed: ", err)
		}
	}

	savedPct := 0.0
	if before > 0 {
		savedPct = float64(before-after) / float64(before) * 100
	}

	return CompactionResult{
		Before:       before,
		After:        after,
		SavedPct:     savedPct,
		Summary:      summary,
		FallbackUsed: false,
	}, nil
}

func estimateTokens(msgs []core.Message) int {
	total := 0
	for _, m := range msgs {
		total += len(m.Text())
	}
	// Rough heuristic: ~4 chars per token for English text.
	return total / 4
}

func (s *ChatService) GetSuggestedMessages(ctx context.Context, ws *models.Workspace) ([]string, error) {
	return []string{"Tell me more", "Can you summarize?", "What are the key points?"}, nil
}

func (s *ChatService) DeleteWorkspaceChats(ctx context.Context, workspaceID int) error {
	var ids []int
	if err := s.db.WithContext(ctx).Model(&models.WorkspaceChat{}).Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.db.Exec("INSERT INTO workspace_chat_fts(workspace_chat_fts) VALUES('delete', ?)", id).Error; err != nil {
			mlog.Error("delete chat fts5 failed: ", err)
		}
	}
	return s.db.Where("workspace_id = ? AND thread_id IS NULL", workspaceID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ChatService) SearchWorkspaceChatsFTS5(ctx context.Context, workspaceID int, query string, limit int) ([]models.WorkspaceChat, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	var chats []models.WorkspaceChat
	err := s.db.WithContext(ctx).Raw(`
		SELECT c.id, c.workspace_id, c.prompt, c.response, c.include, c.user_id, c.thread_id, c.api_session_id, c.created_at, c.last_updated_at, c.feedback_score, c.memory_processed
		FROM workspace_chat_fts f
		JOIN workspace_chats c ON c.id = f.rowid
		WHERE c.workspace_id = ? AND workspace_chat_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, workspaceID, query, limit).Scan(&chats).Error
	if err != nil {
		return nil, err
	}
	return chats, nil
}

func (s *ChatService) DeleteWorkspaceEditedChats(ctx context.Context, workspaceID int) error {
	return s.db.Where("workspace_id = ? AND thread_id IS NULL AND prompt != response", workspaceID).Delete(&models.WorkspaceChat{}).Error
}

func (s *ChatService) UpdateChat(ctx context.Context, workspaceID int, chatID int, req dto.UpdateChatRequest) error {
	updates := map[string]any{}
	if req.Response != "" {
		updates["response"] = req.Response
	}
	if req.Include != nil {
		updates["include"] = *req.Include
	}
	if len(updates) == 0 {
		return fmt.Errorf("no valid fields to update")
	}
	updates["last_updated_at"] = time.Now()
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ? AND workspace_id = ?", chatID, workspaceID).Updates(updates).Error
}

func (s *ChatService) UpdateChatFeedback(ctx context.Context, chatID int, score *bool) error {
	return s.db.Model(&models.WorkspaceChat{}).Where("id = ?", chatID).Update("feedback_score", score).Error
}
