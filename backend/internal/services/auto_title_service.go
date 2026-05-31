package services

import (
	"context"
	"strings"
	"time"

	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/providers"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
	"gorm.io/gorm"
)

const (
	autoTitleMaxLen      = 80
	autoTitlePromptLimit = 500
	autoTitleRespLimit   = 500
	autoTitleTemp        = 0.3
)

// AutoTitleService generates thread titles asynchronously after the first exchange.
type AutoTitleService struct {
	db      *gorm.DB
	llmProv providers.LLMProvider
}

// NewAutoTitleService creates a new auto-title service.
func NewAutoTitleService(db *gorm.DB, llmProv providers.LLMProvider) *AutoTitleService {
	return &AutoTitleService{db: db, llmProv: llmProv}
}

// MaybeGenerate checks if a thread title should be auto-generated and fires a
// background goroutine when the thread is still on its first exchange (≤2 user
// messages) and has no custom title yet.
func (s *AutoTitleService) MaybeGenerate(ctx context.Context, threadID int, userMessage, assistantResponse string) {
	// Synchronous guard checks before spawning a goroutine.
	var thread models.WorkspaceThread
	if err := s.db.WithContext(ctx).First(&thread, threadID).Error; err != nil {
		mlog.Warning("AutoTitle: thread not found", mlog.Int("threadID", threadID), mlog.Err(err))
		return
	}
	// Skip if the user has already set a custom title.
	if thread.Name != "" && thread.Name != "Thread" {
		return
	}

	// Count user messages in this thread.
	var userMsgCount int64
	if err := s.db.WithContext(ctx).Model(&models.WorkspaceChat{}).
		Where("thread_id = ?", threadID).
		Count(&userMsgCount).Error; err != nil {
		mlog.Warning("AutoTitle: failed to count user messages", mlog.Int("threadID", threadID), mlog.Err(err))
		return
	}
	// Only generate on the first exchange (≤2 user messages).
	if userMsgCount > 2 {
		return
	}

	// Fire-and-forget background generation.
	go s.generate(threadID, userMessage, assistantResponse)
}

func (s *AutoTitleService) generate(threadID int, userMessage, assistantResponse string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := buildTitlePrompt(userMessage, assistantResponse)
	temp := autoTitleTemp

	title, err := s.llmProv.Complete(ctx, []core.Message{
		core.NewTextMessage(core.MESSAGE_ROLE_USER, prompt),
	}, "", &temp)
	if err != nil {
		mlog.Warning("AutoTitle: LLM generation failed", mlog.Int("threadID", threadID), mlog.Err(err))
		return
	}

	title = sanitizeTitle(title)
	if title == "" {
		mlog.Warning("AutoTitle: sanitized title is empty", mlog.Int("threadID", threadID))
		return
	}

	if err := s.db.Model(&models.WorkspaceThread{}).Where("id = ?", threadID).Update("name", title).Error; err != nil {
		mlog.Warning("AutoTitle: failed to update thread name", mlog.Int("threadID", threadID), mlog.Err(err))
		return
	}

	mlog.Info("AutoTitle: generated title", mlog.Int("threadID", threadID), mlog.String("title", title))
}

func buildTitlePrompt(userMessage, assistantResponse string) string {
	um := userMessage
	if len(um) > autoTitlePromptLimit {
		um = um[:autoTitlePromptLimit] + "..."
	}
	ar := assistantResponse
	if len(ar) > autoTitleRespLimit {
		ar = ar[:autoTitleRespLimit] + "..."
	}

	return `You are a helpful assistant that generates short, descriptive chat titles.

Based on the following conversation exchange, generate a concise title of 3-7 words that captures the main topic or intent.

User: ` + um + `

Assistant: ` + ar + `

Return ONLY the title text, nothing else. No quotes, no punctuation at the end, no prefixes. The title should be in the same language as the user's message.`
}

func sanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	// Remove common prefixes.
	title = strings.TrimPrefix(title, "Title:")
	title = strings.TrimPrefix(title, "title:")
	title = strings.TrimSpace(title)
	// Remove surrounding quotes.
	title = strings.Trim(title, `"'`)
	title = strings.TrimSpace(title)
	// Truncate.
	if len(title) > autoTitleMaxLen {
		title = title[:autoTitleMaxLen-3] + "..."
	}
	return title
}
