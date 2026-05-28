package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/config"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/embedder"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/hermind/backend/internal/reranker"
	"github.com/odysseythink/hermind/backend/internal/vectordb"
	"github.com/odysseythink/hermind/backend/pkg/utils"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

type EmbedService struct {
	db        *gorm.DB
	cfg       *config.Config
	vectorSvc *VectorService
	llmProv   providers.LLMProvider
	embedder  embedder.Embedder
	reranker  reranker.Reranker
}

func NewEmbedService(
	db *gorm.DB,
	cfg *config.Config,
	vectorSvc *VectorService,
	llmProv providers.LLMProvider,
	embedder embedder.Embedder,
	reranker reranker.Reranker,
) *EmbedService {
	return &EmbedService{db, cfg, vectorSvc, llmProv, embedder, reranker}
}

func (s *EmbedService) Create(ctx context.Context, req dto.CreateEmbedConfigRequest, creatorID *int) (*models.EmbedConfig, error) {
	var ws models.Workspace
	if err := s.db.Where("slug = ?", req.WorkspaceSlug).First(&ws).Error; err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	chatMode := req.ChatMode
	if chatMode != "chat" && chatMode != "query" {
		chatMode = "query"
	}

	domainsJSON, err := normalizeAllowlistDomains(req.AllowlistDomains)
	if err != nil {
		return nil, fmt.Errorf("invalid allowlist domains: %w", err)
	}

	msgLimit := 20
	if req.MessageLimit != nil && *req.MessageLimit > 0 {
		msgLimit = *req.MessageLimit
	}

	embed := models.EmbedConfig{
		UUID:                     uuid.New().String(),
		Enabled:                  true,
		ChatMode:                 chatMode,
		AllowlistDomains:         domainsJSON,
		AllowModelOverride:       req.AllowModelOverride,
		AllowTemperatureOverride: req.AllowTemperatureOverride,
		AllowPromptOverride:      req.AllowPromptOverride,
		MaxChatsPerDay:           positiveOrNil(req.MaxChatsPerDay),
		MaxChatsPerSession:       positiveOrNil(req.MaxChatsPerSession),
		MessageLimit:             &msgLimit,
		WorkspaceID:              ws.ID,
		CreatedBy:                creatorID,
		CreatedAt:                time.Now(),
		LastUpdatedAt:            time.Now(),
	}

	if err := s.db.Create(&embed).Error; err != nil {
		return nil, fmt.Errorf("create embed config: %w", err)
	}
	return &embed, nil
}

func normalizeAllowlistDomains(domains []string) (*string, error) {
	if domains == nil {
		return nil, nil
	}
	for i, d := range domains {
		if !strings.HasPrefix(d, "http://") && !strings.HasPrefix(d, "https://") {
			d = "https://" + d
		}
		if _, err := url.Parse(d); err != nil {
			return nil, err
		}
		domains[i] = d
	}
	b, _ := json.Marshal(domains)
	s := string(b)
	return &s, nil
}

func positiveOrNil(v *int) *int {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}

func (s *EmbedService) Update(ctx context.Context, embedID int, req dto.UpdateEmbedConfigRequest) error {
	updates := map[string]any{}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.ChatMode != nil {
		if *req.ChatMode == "chat" || *req.ChatMode == "query" {
			updates["chat_mode"] = *req.ChatMode
		}
	}
	if req.AllowlistDomains != nil {
		jsonStr, err := normalizeAllowlistDomains(req.AllowlistDomains)
		if err != nil {
			return err
		}
		updates["allowlist_domains"] = jsonStr
	}
	if req.AllowModelOverride != nil {
		updates["allow_model_override"] = *req.AllowModelOverride
	}
	if req.AllowTemperatureOverride != nil {
		updates["allow_temperature_override"] = *req.AllowTemperatureOverride
	}
	if req.AllowPromptOverride != nil {
		updates["allow_prompt_override"] = *req.AllowPromptOverride
	}
	if req.MaxChatsPerDay != nil {
		updates["max_chats_per_day"] = positiveOrNil(req.MaxChatsPerDay)
	}
	if req.MaxChatsPerSession != nil {
		updates["max_chats_per_session"] = positiveOrNil(req.MaxChatsPerSession)
	}
	if req.MessageLimit != nil && *req.MessageLimit > 0 {
		updates["message_limit"] = *req.MessageLimit
	}
	if req.WorkspaceID != nil {
		updates["workspace_id"] = *req.WorkspaceID
	}
	updates["last_updated_at"] = time.Now()

	return s.db.WithContext(ctx).Model(&models.EmbedConfig{}).Where("id = ?", embedID).Updates(updates).Error
}

func (s *EmbedService) Delete(ctx context.Context, embedID int) error {
	return s.db.WithContext(ctx).Delete(&models.EmbedConfig{}, embedID).Error
}

func (s *EmbedService) GetByUUID(ctx context.Context, uuid string) (*models.EmbedConfig, error) {
	var embed models.EmbedConfig
	if err := s.db.WithContext(ctx).Where("uuid = ?", uuid).Preload("Workspace").First(&embed).Error; err != nil {
		return nil, err
	}
	return &embed, nil
}

func (s *EmbedService) GetByID(ctx context.Context, id int) (*models.EmbedConfig, error) {
	var embed models.EmbedConfig
	if err := s.db.WithContext(ctx).First(&embed, id).Error; err != nil {
		return nil, err
	}
	return &embed, nil
}

func (s *EmbedService) List(ctx context.Context) ([]dto.EmbedConfigResponse, error) {
	var configs []models.EmbedConfig
	if err := s.db.WithContext(ctx).Preload("Workspace").Order("created_at DESC").Find(&configs).Error; err != nil {
		return nil, err
	}

	var resp []dto.EmbedConfigResponse
	for _, cfg := range configs {
		var count int64
		s.db.Model(&models.EmbedChat{}).Where("embed_id = ?", cfg.ID).Count(&count)
		resp = append(resp, dto.EmbedConfigResponse{
			ID:        cfg.ID,
			UUID:      cfg.UUID,
			Enabled:   cfg.Enabled,
			ChatMode:  cfg.ChatMode,
			Workspace: dto.WorkspaceSummary{ID: cfg.Workspace.ID, Name: cfg.Workspace.Name},
			ChatCount: count,
			CreatedAt: cfg.CreatedAt,
		})
	}
	return resp, nil
}

func (s *EmbedService) ListChats(ctx context.Context, embedID int, sessionID *string, limit, offset int) ([]models.EmbedChat, error) {
	var chats []models.EmbedChat
	q := s.db.WithContext(ctx).Where("embed_id = ?", embedID)
	if sessionID != nil {
		q = q.Where("session_id = ?", *sessionID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	if err := q.Order("id DESC").Find(&chats).Error; err != nil {
		return nil, err
	}
	return chats, nil
}

func (s *EmbedService) MarkHistoryInvalid(ctx context.Context, embedID int, sessionID string) error {
	return s.db.WithContext(ctx).
		Model(&models.EmbedChat{}).
		Where("embed_id = ? AND session_id = ?", embedID, sessionID).
		Update("include", false).Error
}

func (s *EmbedService) CountRecentChats(ctx context.Context, embedID int, since time.Time) int64 {
	var count int64
	s.db.WithContext(ctx).Model(&models.EmbedChat{}).
		Where("embed_id = ? AND created_at >= ?", embedID, since).
		Count(&count)
	return count
}

func (s *EmbedService) CountRecentSessionChats(ctx context.Context, embedID int, sessionID string, since time.Time) int64 {
	var count int64
	s.db.WithContext(ctx).Model(&models.EmbedChat{}).
		Where("embed_id = ? AND session_id = ? AND created_at >= ?", embedID, sessionID, since).
		Count(&count)
	return count
}

func (s *EmbedService) CountAllChats(ctx context.Context, total *int64) error {
	return s.db.WithContext(ctx).Model(&models.EmbedChat{}).Count(total).Error
}

func (s *EmbedService) ListAllChatsPaginated(ctx context.Context, limit, offset int) ([]dto.EmbedChatAdminItem, error) {
	var chats []models.EmbedChat
	if err := s.db.WithContext(ctx).
		Preload("EmbedConfig", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "uuid")
		}).
		Order("id DESC").
		Limit(limit).Offset(offset).Find(&chats).Error; err != nil {
		return nil, err
	}

	var resp []dto.EmbedChatAdminItem
	for _, chat := range chats {
		var ws models.Workspace
		s.db.First(&ws, "id = (SELECT workspace_id FROM embed_configs WHERE id = ?)", chat.EmbedID)
		resp = append(resp, dto.EmbedChatAdminItem{
			ID:          chat.ID,
			Prompt:      chat.Prompt,
			Response:    chat.Response,
			SessionID:   chat.SessionID,
			EmbedConfig: dto.EmbedConfigShort{ID: chat.EmbedID, UUID: chat.EmbedConfig.UUID},
			Workspace:   dto.WorkspaceSummary{ID: ws.ID, Name: ws.Name},
			CreatedAt:   chat.CreatedAt,
		})
	}
	return resp, nil
}

func (s *EmbedService) DeleteChat(ctx context.Context, chatID int) error {
	return s.db.WithContext(ctx).Delete(&models.EmbedChat{}, chatID).Error
}

func (s *EmbedService) StreamChat(ctx context.Context, embed *models.EmbedConfig, req *dto.EmbedStreamChatRequest, conn *dto.ConnectionMeta) (<-chan dto.StreamChatResponse, error) {
	out := make(chan dto.StreamChatResponse, 16)
	uuidStr := uuid.New().String()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("internal error: %v", r)
				out <- dto.StreamChatResponse{UUID: uuid.New().String(), Type: "abort", Close: true, Error: &msg}
				close(out)
			}
		}()
		defer close(out)
		var fullText strings.Builder

		// Ensure workspace is loaded
		ws := embed.Workspace
		if ws.ID == 0 {
			if err := s.db.WithContext(ctx).First(&ws, embed.WorkspaceID).Error; err != nil {
				out <- dto.StreamChatResponse{
					UUID:  uuidStr,
					Type:  "abort",
					Close: true,
					Error: utils.Ptr("workspace not found"),
				}
				return
			}
		}

		// 1. Resolve chat mode
		chatMode := embed.ChatMode
		if chatMode == "automatic" {
			chatMode = "chat"
		}

		// 2. Apply overrides (only if config permits)
		systemPrompt := ""
		if ws.OpenAiPrompt != nil {
			systemPrompt = *ws.OpenAiPrompt
		}
		if embed.AllowPromptOverride && req.Prompt != nil {
			systemPrompt = *req.Prompt
		}
		var temperatureOverride *float64
		if embed.AllowTemperatureOverride && req.Temperature != nil {
			temperatureOverride = req.Temperature
		}

		// 3. Query mode empty-doc guard
		if chatMode == "query" {
			var vectorCount int64
			s.db.Model(&models.DocumentVector{}).
				Joins("JOIN workspace_documents ON workspace_documents.doc_id = document_vectors.doc_id").
				Where("workspace_documents.workspace_id = ?", embed.WorkspaceID).
				Count(&vectorCount)
			if vectorCount == 0 {
				msg := "I do not have enough information to answer that. Try another question."
				out <- dto.StreamChatResponse{
					UUID:         uuidStr,
					Type:         "textResponseChunk",
					TextResponse: &msg,
					Close:        true,
				}
				return
			}
		}

		// 4. Load history
		msgLimit := 20
		if embed.MessageLimit != nil {
			msgLimit = *embed.MessageLimit
		}
		chats, err := s.ListChats(ctx, embed.ID, &req.SessionID, msgLimit, 0)
		if err != nil {
			out <- dto.StreamChatResponse{
				UUID:  uuidStr,
				Type:  "abort",
				Close: true,
				Error: utils.Ptr(err.Error()),
			}
			return
		}

		var history []core.Message
		for i := len(chats) - 1; i >= 0; i-- {
			c := chats[i]
			if !c.Include {
				continue
			}
			respText := c.Response
			var respObj map[string]interface{}
			if err := json.Unmarshal([]byte(c.Response), &respObj); err == nil {
				if text, ok := respObj["text"].(string); ok {
					respText = text
				}
			}
			history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_USER, c.Prompt))
			history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, respText))
		}

		// 5. Pinned docs
		var contextTexts []string
		var sources []any

		var pinnedDocs []models.WorkspaceDocument
		s.db.Where("workspace_id = ? AND pinned = ?", embed.WorkspaceID, true).Find(&pinnedDocs)
		for _, doc := range pinnedDocs {
			content := s.readDocumentContent(&doc)
			if content != "" {
				contextTexts = append(contextTexts, content)
				sources = append(sources, map[string]any{
					"docId":    doc.DocId,
					"text":     content,
					"filename": doc.Filename,
				})
			}
		}

		// 6. Vector search
		if s.vectorSvc != nil && s.vectorSvc.provider != nil {
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
				qv, err := s.embedder.EmbedQuery(ctx, req.Message)
				if err == nil {
					queryVector = qv
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
					if ranked, err := s.reranker.Rerank(ctx, req.Message, texts, topN); err == nil {
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
				for _, r := range results {
					contextTexts = append(contextTexts, r.Text)
					sources = append(sources, map[string]any{
						"docId":    r.DocId,
						"text":     r.Text,
						"score":    r.Score,
						"metadata": r.Metadata,
					})
				}
			}
		}

		// 7. Query mode no-context guard
		if chatMode == "query" && len(contextTexts) == 0 {
			msg := "There is no relevant information to answer this question."
			if ws.QueryRefusalResponse != nil {
				msg = *ws.QueryRefusalResponse
			}
			out <- dto.StreamChatResponse{
				UUID:         uuidStr,
				Type:         "textResponseChunk",
				TextResponse: &msg,
				Close:        true,
			}
			return
		}

		// 8. Build messages and stream
		if len(contextTexts) > 0 {
			if systemPrompt != "" {
				systemPrompt += "\n\nContext:\n" + strings.Join(contextTexts, "\n---\n")
			} else {
				systemPrompt = "Context:\n" + strings.Join(contextTexts, "\n---\n")
			}
		}

		messages := append(history, core.Message{
			Role:    core.MESSAGE_ROLE_USER,
			Content: core.NewTextContent(req.Message),
		})

		if s.llmProv == nil {
			out <- dto.StreamChatResponse{
				UUID:  uuidStr,
				Type:  "abort",
				Close: true,
				Error: utils.Ptr("no LLM provider configured"),
			}
			return
		}

		chunks, err := s.llmProv.Stream(ctx, messages, systemPrompt, temperatureOverride)
		if err != nil {
			out <- dto.StreamChatResponse{
				UUID:  uuidStr,
				Type:  "abort",
				Close: true,
				Error: utils.Ptr(err.Error()),
			}
			return
		}

		for chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if chunk.Err != nil {
				out <- dto.StreamChatResponse{
					UUID:  uuidStr,
					Type:  "abort",
					Close: true,
					Error: utils.Ptr(chunk.Err.Error()),
				}
				return
			}

			if chunk.TextDelta != "" {
				fullText.WriteString(chunk.TextDelta)
				out <- dto.StreamChatResponse{
					UUID:         uuidStr,
					Type:         "textResponseChunk",
					TextResponse: utils.Ptr(chunk.TextDelta),
					Close:        false,
				}
			}

			if chunk.FinishReason != "" {
				out <- dto.StreamChatResponse{
					UUID:         uuidStr,
					Type:         "finalizeResponseStream",
					TextResponse: utils.Ptr(""),
					Close:        true,
					Sources:      sources,
				}
				s.saveEmbedChat(ctx, embed, req, conn, fullText.String(), chatMode, sources)
				return
			}
		}

		// Stream ended without finish reason
		out <- dto.StreamChatResponse{
			UUID:         uuidStr,
			Type:         "finalizeResponseStream",
			TextResponse: utils.Ptr(""),
			Close:        true,
			Sources:      sources,
		}
		s.saveEmbedChat(ctx, embed, req, conn, fullText.String(), chatMode, sources)
	}()

	return out, nil
}

func (s *EmbedService) saveEmbedChat(ctx context.Context, embed *models.EmbedConfig, req *dto.EmbedStreamChatRequest, conn *dto.ConnectionMeta, fullText, chatMode string, sources []any) {
	respObj := map[string]any{
		"text":    fullText,
		"type":    chatMode,
		"sources": sources,
		"metrics": map[string]any{},
	}
	respJSON, err := json.Marshal(respObj)
	if err != nil {
		respJSON = []byte(`{"text":""}`)
	}

	connObj := map[string]any{
		"host":     conn.Host,
		"ip":       conn.IP,
		"username": req.Username,
	}
	connJSON, err := json.Marshal(connObj)
	if err != nil {
		connJSON = []byte(`{}`)
	}
	connStr := string(connJSON)

	chat := models.EmbedChat{
		EmbedID:               embed.ID,
		SessionID:             req.SessionID,
		Prompt:                req.Message,
		Response:              string(respJSON),
		ConnectionInformation: &connStr,
		Include:               true,
		CreatedAt:             time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&chat).Error; err != nil {
		// Log persistence failure but don't fail the user-facing response
		fmt.Printf("failed to save embed chat: %v\n", err)
	}
}

func (s *EmbedService) readDocumentContent(doc *models.WorkspaceDocument) string {
	if doc.Metadata != nil {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(*doc.Metadata), &meta); err == nil {
			if extracted, ok := meta["extractedText"].(string); ok && extracted != "" {
				return extracted
			}
		}
	}
	data, err := os.ReadFile(doc.Docpath)
	if err != nil {
		return ""
	}
	// Try parsing as JSON with pageContent (Node.js compatibility)
	var jsonDoc struct {
		PageContent string `json:"pageContent"`
	}
	if err := json.Unmarshal(data, &jsonDoc); err == nil && jsonDoc.PageContent != "" {
		return jsonDoc.PageContent
	}
	return string(data)
}
