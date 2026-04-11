package gateway

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/nousresearch/hermes-agent/agent"
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/message"
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
}

// NewGateway builds a Gateway with the given dependencies.
func NewGateway(cfg config.Config, p, aux provider.Provider, s storage.Storage, reg *tool.Registry) *Gateway {
	return &Gateway{
		cfg:       cfg,
		provider:  p,
		aux:       aux,
		storage:   s,
		tools:     reg,
		platforms: make(map[string]Platform),
		sessions:  NewSessionStore(),
	}
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
			log.Printf("gateway: starting platform %s", name)
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

// handleMessage is the MessageHandler passed to each platform.
func (g *Gateway) handleMessage(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	sess := g.sessions.GetOrCreate(in.Platform, in.UserID)

	eng := agent.NewEngineWithToolsAndAux(
		g.provider, g.aux, g.storage, g.tools,
		g.cfg.Agent, in.Platform,
	)
	// Copy the session history so the Engine can't mutate the
	// cached slice concurrently with another request.
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

// modelFromCfg extracts the model name from cfg.Model (strip provider/ prefix).
func modelFromCfg(cfg config.Config) string {
	if idx := strings.Index(cfg.Model, "/"); idx >= 0 {
		return cfg.Model[idx+1:]
	}
	return cfg.Model
}
