// Package gatewayctl owns the gateway lifecycle in processes that also
// serve the REST API. Apply stop-restarts the gateway subsystem with
// the latest in-memory config; TestPlatform runs a descriptor probe.
package gatewayctl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
)

// Sentinel errors a caller may Is-check. These ALIAS api package
// sentinels so handler errors.Is works across the interface boundary.
// (`errors` import is still required — Start uses errors.New below.)
var (
	ErrApplyInProgress    = api.ErrApplyInProgress
	ErrUnknownKey         = api.ErrUnknownPlatformKey
	ErrTestNotImplemented = api.ErrTestNotImplemented
)

// GatewayBuilder builds a Gateway from a config. Typically set to cli.BuildGateway.
type GatewayBuilder func(cfg config.Config) (*gateway.Gateway, error)

// Controller manages the lifecycle of a single gateway.Gateway using
// the mutable config pointer it was given at construction time.
type Controller struct {
	cfg     *config.Config
	builder GatewayBuilder

	// mu is a short-lived guard over g. Held only while reading or
	// writing the pointer.
	mu sync.Mutex
	g  *gateway.Gateway

	// applyMu serializes Apply with itself and with Shutdown. Held for
	// the whole rebuild cycle so Shutdown cannot race a half-swapped
	// gateway. Running() intentionally does NOT take this lock —
	// during an in-flight Apply it may briefly report the names of
	// the gateway being replaced. Acceptable for the UI status strip.
	applyMu sync.Mutex
}

// New returns a Controller bound to the given config pointer and builder function.
// The config pointer is used live — callers must not swap it out and must
// serialize mutations through PUT /api/config.
func New(cfg *config.Config, builder GatewayBuilder) *Controller {
	return &Controller{cfg: cfg, builder: builder}
}

// Start builds and runs the initial Gateway in a background goroutine.
// Calling Start twice on an already-running controller returns an error.
// Start does not block on Gateway.Start returning — that blocks forever
// under normal operation.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.g != nil {
		return errors.New("controller: already started")
	}
	g, err := c.builder(*c.cfg)
	if err != nil {
		return err
	}
	c.g = g
	if len(g.Names()) > 0 {
		go func() { _ = g.Start(context.Background()) }()
	}
	return nil
}

// Running returns the sorted names of currently-running platforms.
func (c *Controller) Running() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.g == nil {
		return nil
	}
	return c.g.Names()
}

// Shutdown stops the current Gateway. Serialized with Apply via
// applyMu so it cannot race a half-swapped gateway. Callers should
// ensure no further Apply calls happen after Shutdown returns.
func (c *Controller) Shutdown(ctx context.Context) {
	c.applyMu.Lock()
	defer c.applyMu.Unlock()
	c.mu.Lock()
	g := c.g
	c.mu.Unlock()
	if g != nil {
		_ = g.Stop(ctx)
	}
}

// Apply stops the current Gateway, rebuilds it from c.cfg, and starts
// the new one. A second concurrent Apply returns ErrApplyInProgress.
// Returns api.ApplyResult so the HTTP handler can write it through
// directly without a second mapping layer. Apply does not wait for
// the new Gateway to finish starting — Gateway.Start blocks forever
// under normal operation.
func (c *Controller) Apply(ctx context.Context) (api.ApplyResult, error) {
	if !c.applyMu.TryLock() {
		return api.ApplyResult{}, ErrApplyInProgress
	}
	defer c.applyMu.Unlock()

	start := time.Now()

	c.mu.Lock()
	old := c.g
	c.mu.Unlock()

	if old != nil {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = old.Stop(stopCtx)
		cancel()
	}

	built, err := c.builder(*c.cfg)
	if err != nil {
		return api.ApplyResult{OK: false, TookMS: time.Since(start).Milliseconds()},
			fmt.Errorf("rebuild: %w", err)
	}

	c.mu.Lock()
	c.g = built
	c.mu.Unlock()

	names := built.Names()
	if len(names) > 0 {
		go func() { _ = built.Start(context.Background()) }()
	}

	return api.ApplyResult{
		OK:        true,
		Restarted: names,
		TookMS:    time.Since(start).Milliseconds(),
	}, nil
}

// TestPlatform runs descriptor.Test for the platform stored under key.
// Uses a 10s deadline. Returns ErrUnknownKey if the key is not in
// c.cfg.Gateway.Platforms; ErrTestNotImplemented if the descriptor
// has no Test closure (e.g. Stage 2a placeholder); otherwise propagates
// whatever descriptor.Test returned.
func (c *Controller) TestPlatform(ctx context.Context, key string) error {
	pc, ok := c.cfg.Gateway.Platforms[key]
	if !ok {
		return ErrUnknownKey
	}
	d, ok := platforms.Get(pc.Type)
	if !ok {
		return fmt.Errorf("unknown platform type %q", pc.Type)
	}
	if d.Test == nil {
		return ErrTestNotImplemented
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return d.Test(tctx, pc.Options)
}
