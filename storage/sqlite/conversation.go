// storage/sqlite/conversation.go
package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseythink/hermind/storage"
)

// UpdateSystemPromptCache stores the current system prompt so Anthropic
// prefix caching can reuse it across turns.
func (s *Store) UpdateSystemPromptCache(ctx context.Context, prompt string) error {
	return updateSystemPromptCacheExec(ctx, s.db, prompt)
}

func (t *txImpl) UpdateSystemPromptCache(ctx context.Context, prompt string) error {
	return updateSystemPromptCacheExec(ctx, t.tx, prompt)
}

func updateSystemPromptCacheExec(ctx context.Context, ex execer, prompt string) error {
	_, err := ex.ExecContext(ctx, `
		UPDATE conversation_state
		SET system_prompt_cache = ?, updated_at = ?
		WHERE id = 1`,
		prompt, toEpoch(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("sqlite: update system_prompt_cache: %w", err)
	}
	return nil
}

// UpdateUsage adds a usage delta to the singleton conversation_state row.
func (s *Store) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error {
	return updateUsageExec(ctx, s.db, u)
}

func (t *txImpl) UpdateUsage(ctx context.Context, u *storage.UsageUpdate) error {
	return updateUsageExec(ctx, t.tx, u)
}

func updateUsageExec(ctx context.Context, ex execer, u *storage.UsageUpdate) error {
	if u == nil {
		return nil
	}
	_, err := ex.ExecContext(ctx, `
		UPDATE conversation_state SET
		  total_input_tokens = total_input_tokens + ?,
		  total_output_tokens = total_output_tokens + ?,
		  total_cache_read_tokens = total_cache_read_tokens + ?,
		  total_cache_write_tokens = total_cache_write_tokens + ?,
		  total_cost_usd = total_cost_usd + ?,
		  updated_at = ?
		WHERE id = 1`,
		u.InputTokens, u.OutputTokens,
		u.CacheReadTokens, u.CacheWriteTokens,
		u.CostUSD, toEpoch(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("sqlite: update usage: %w", err)
	}
	return nil
}
