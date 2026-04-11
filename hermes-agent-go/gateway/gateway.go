package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/logging"
	"github.com/nousresearch/hermes-agent/message"
	"github.com/nousresearch/hermes-agent/metrics"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/storage"
	"github.com/nousresearch/hermes-agent/tool"
)

// Gateway routes messages from one or more Platform adapters into
// per-user agent.Engine conversations.
type Gateway struct {
	cfg       config.Config
	provider  provider.Provider
	aux       provider.Provider
	storage   storage.Storage
	tools     *tool.Registry
	platforms map[string]Platform
	sessions  *SessionStore
	dedup     *Dedup

	// Metrics — nil means metrics are disabled.
	metrics          *metrics.Registry
	metricMessages   *metrics.Counter
	metricErrors     *metrics.Counter
	metricRetries    *metrics.Counter
	metricHandlerDur *metrics.Histogram
}

// NewGateway builds a Gateway with the given dependencies.
func NewGateway(cfg config.Config, p, aux provider.Provider, s storage.Storage, reg *tool.Registry) *Gateway {
	g := &Gateway{
		cfg:       cfg,
		provider:  p,
		aux:       aux,
		storage:   s,
		tools:     reg,
		platforms: make(map[string]Platform),
		sessions:  NewSessionStore(),
		dedup:     NewDedup(2048),
	}
	if s != nil {
		g.sessions.SetLoader(g.loadHistoryFromStorage)
	}
	return g
}

// SetMetrics attaches a metrics registry and registers the standard
// gateway metrics into it. Safe to call at most once; subsequent
// calls are ignored.
func (g *Gateway) SetMetrics(reg *metrics.Registry) {
	if g.metrics != nil || reg == nil {
		return
	}
	g.metrics = reg
	g.metricMessages = reg.NewCounter("gateway_messages_total", "Total inbound messages.")
	g.metricErrors = reg.NewCounter("gateway_handler_errors_total", "Total handler errors (final, after retries).")
	g.metricRetries = reg.NewCounter("gateway_handler_retry_total", "Total handler retry attempts.")
	g.metricHandlerDur = reg.NewHistogram("gateway_handler_duration_seconds", "Handler duration in seconds.")
}

// Register adds a platform adapter. Duplicate names replace prior entries.
func (g *Gateway) Register(p Platform) {
	g.platforms[p.Name()] = p
}

// Start runs all registered platforms in their own goroutines and
// blocks until ctx is done or any Platform.Run returns a non-nil error.
func (g *Gateway) Start(ctx context.Context) error {
	if len(g.platforms) == 0 {
		return fmt.Errorf("gateway: no platforms registered")
	}
	errCh := make(chan error, len(g.platforms))
	var wg sync.WaitGroup

	for name, p := range g.platforms {
		wg.Add(1)
		go func(name string, p Platform) {
			defer wg.Done()
			slog.InfoContext(ctx, "gateway: starting platform", "platform", name)
			if err := p.Run(ctx, g.handleMessage); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("gateway: %s: %w", name, err)
			}
		}(name, p)
	}

	select {
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// handleMessage is the MessageHandler passed to each platform. It
// attaches a request ID, performs deduplication, and invokes the
// retry helper.
func (g *Gateway) handleMessage(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	ctx = logging.WithRequestID(ctx, "")
	if in.MessageID != "" {
		key := in.Platform + ":" + in.MessageID
		if g.dedup.Seen(key) {
			slog.InfoContext(ctx, "gateway: duplicate message skipped", "platform", in.Platform, "message_id", in.MessageID)
			return nil, nil
		}
	}
	if g.metricMessages != nil {
		g.metricMessages.With(map[string]string{"platform": in.Platform}).Inc()
	}
	start := time.Now()
	out, err := g.runWithRetry(ctx, in)
	if g.metricHandlerDur != nil {
		g.metricHandlerDur.With(map[string]string{"platform": in.Platform}).Observe(time.Since(start).Seconds())
	}
	if err != nil && g.metricErrors != nil {
		g.metricErrors.With(map[string]string{"platform": in.Platform}).Inc()
	}
	return out, err
}

// runWithRetry executes runOnce up to maxAttempts times with
// exponential backoff between attempts.
func (g *Gateway) runWithRetry(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	const maxAttempts = 3
	var lastErr error
	backoff := 100 * time.Millisecond
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, err := g.runOnce(ctx, in)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err
		slog.WarnContext(ctx, "gateway: retry", "attempt", attempt, "err", err.Error())
		if g.metricRetries != nil {
			g.metricRetries.With(map[string]string{"platform": in.Platform}).Inc()
		}
		if attempt == maxAttempts {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}
	return nil, lastErr
}

// runOnce performs a single Engine invocation for the given incoming
// message and returns the reply.
func (g *Gateway) runOnce(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	sess := g.sessions.GetOrCreate(ctx, in.Platform, in.UserID)

	eng := agent.NewEngineWithToolsAndAux(
		g.provider, g.aux, g.storage, g.tools,
		g.cfg.Agent, in.Platform,
	)
	historyCopy := append([]message.Message{}, sess.History...)
	result, err := eng.RunConversation(ctx, &agent.RunOptions{
		UserMessage: in.Text,
		History:     historyCopy,
		SessionID:   sess.ID,
		UserID:      in.UserID,
		Model:       modelFromCfg(g.cfg),
	})
	if err != nil {
		return nil, err
	}
	g.sessions.SetHistory(sess, result.Messages)
	return &OutgoingMessage{
		UserID: in.UserID,
		ChatID: in.ChatID,
		Text:   result.Response.Content.Text(),
	}, nil
}

// loadHistoryFromStorage rehydrates a session's history from the
// persistent message store. Only the last 50 messages are loaded to
// bound rehydration cost.
func (g *Gateway) loadHistoryFromStorage(ctx context.Context, platform, userID, sessionID string) ([]message.Message, error) {
	if g.storage == nil {
		return nil, nil
	}
	// GetSession first — if the session row doesn't exist, there is
	// no persisted history yet.
	if _, err := g.storage.GetSession(ctx, sessionID); err != nil {
		return nil, nil
	}
	stored, err := g.storage.GetMessages(ctx, sessionID, 50, 0)
	if err != nil {
		return nil, err
	}
	out := make([]message.Message, 0, len(stored))
	for _, sm := range stored {
		var content message.Content
		if err := json.Unmarshal([]byte(sm.Content), &content); err != nil {
			// Fallback: treat as plain text.
			content = message.TextContent(sm.Content)
		}
		out = append(out, message.Message{
			Role:    message.Role(sm.Role),
			Content: content,
		})
	}
	return out, nil
}

// modelFromCfg extracts the model name from cfg.Model (strip provider/ prefix).
func modelFromCfg(cfg config.Config) string {
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		return cfg.Model[idx+1:]
	}
	return cfg.Model
}
