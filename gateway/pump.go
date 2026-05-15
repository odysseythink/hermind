package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/mlog"
)

// Runner is implemented by the API server; Pump calls it to process
// each incoming message against the shared single-conversation engine.
type Runner interface {
	RunTurn(ctx context.Context, userMessage string) (string, error)
}

// Builder creates a Platform from the per-instance options map.
type Builder func(instanceName string, opts map[string]string) (Platform, error)

var (
	builderMu sync.RWMutex
	builders  = map[string]Builder{}
)

// RegisterBuilder registers a builder for the given platform type.
// Called from init() in each platform source file.
func RegisterBuilder(typ string, b Builder) {
	builderMu.Lock()
	defer builderMu.Unlock()
	builders[typ] = b
}

// Pump starts all enabled platforms in config and routes messages to
// the shared runner.
type Pump struct {
	runner    Runner
	platforms map[string]Platform // instance name → adapter
	dedup     *Dedup
}

// NewPump constructs a Pump from the gateway config section.
// Only platforms with Enabled=true and a known Type are started.
// Returns (nil, nil) when no platforms are enabled — callers should
// skip calling Start in that case.
func NewPump(cfg config.GatewayConfig, runner Runner) (*Pump, error) {
	builderMu.RLock()
	defer builderMu.RUnlock()

	p := &Pump{
		runner:    runner,
		platforms: make(map[string]Platform),
		dedup:     NewDedup(2048),
	}

	for name, pcfg := range cfg.Platforms {
		if !pcfg.Enabled {
			mlog.Info("gateway: platform disabled, skipping", mlog.String("name", name), mlog.String("type", pcfg.Type))
			continue
		}
		build, ok := builders[pcfg.Type]
		if !ok {
			mlog.Warning("gateway: unknown platform type", mlog.String("name", name), mlog.String("type", pcfg.Type))
			continue
		}
		pl, err := build(name, pcfg.Options)
		if err != nil {
			return nil, fmt.Errorf("gateway: build %s (%s): %w", name, pcfg.Type, err)
		}
		p.platforms[name] = pl
		mlog.Info("gateway: registered platform", mlog.String("name", name), mlog.String("type", pcfg.Type))
	}

	return p, nil
}

// HasPlatforms reports whether at least one enabled platform was built.
func (p *Pump) HasPlatforms() bool { return len(p.platforms) > 0 }

// Start runs all platforms concurrently. Blocks until ctx is cancelled.
func (p *Pump) Start(ctx context.Context) {
	if !p.HasPlatforms() {
		mlog.Info("gateway: no enabled platforms")
		return
	}
	var wg sync.WaitGroup
	for name, pl := range p.platforms {
		wg.Add(1)
		go func(name string, pl Platform) {
			defer wg.Done()
			mlog.Info("gateway: platform starting", mlog.String("name", name))
			if err := pl.Run(ctx, p.handle); err != nil && ctx.Err() == nil {
				mlog.Error("gateway: platform stopped with error", mlog.String("name", name), mlog.String("err", err.Error()))
			} else {
				mlog.Info("gateway: platform stopped", mlog.String("name", name))
			}
		}(name, pl)
	}
	wg.Wait()
}

func (p *Pump) handle(ctx context.Context, in IncomingMessage) (*OutgoingMessage, error) {
	if in.MessageID != "" {
		key := in.Platform + ":" + in.MessageID
		if p.dedup.Seen(key) {
			mlog.InfoContext(ctx, "gateway: duplicate message skipped",
				mlog.String("platform", in.Platform), mlog.String("msg_id", in.MessageID))
			return nil, nil
		}
	}
	mlog.InfoContext(ctx, "gateway: incoming message",
		mlog.String("platform", in.Platform),
		mlog.String("user_id", in.UserID),
		mlog.String("chat_id", in.ChatID),
		mlog.Int("text_len", len(in.Text)),
	)

	reply, err := p.runner.RunTurn(ctx, in.Text)
	if err != nil {
		mlog.ErrorContext(ctx, "gateway: RunTurn failed",
			mlog.String("platform", in.Platform), mlog.String("err", err.Error()))
		return nil, err
	}

	mlog.InfoContext(ctx, "gateway: reply ready",
		mlog.String("platform", in.Platform),
		mlog.Int("reply_len", len(reply)),
	)
	return &OutgoingMessage{
		UserID: in.UserID,
		ChatID: in.ChatID,
		Text:   reply,
	}, nil
}
