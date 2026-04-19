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
	"github.com/odysseythink/hermind/cli"
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

// Controller manages the lifecycle of a single gateway.Gateway using
// the mutable config pointer it was given at construction time.
type Controller struct {
	cfg *config.Config

	mu      sync.Mutex // guards g + started
	g       *gateway.Gateway
	started chan struct{} // closed after Start returned; nil when not running

	applyMu sync.Mutex
}

// New returns a Controller bound to the given config pointer. The
// pointer is used live — callers must not swap it out and must
// serialize mutations through PUT /api/config.
func New(cfg *config.Config) *Controller {
	return &Controller{cfg: cfg}
}

// Start builds and runs the initial Gateway in a background goroutine.
// Returns once Start has entered its wait loop (best-effort). Start is
// idempotent-ish: calling it twice on an already-running controller
// returns an error.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.g != nil {
		return errors.New("controller: already started")
	}
	g, err := cli.BuildGateway(cli.BuildGatewayDeps{Config: *c.cfg})
	if err != nil {
		return err
	}
	if len(g.Names()) == 0 {
		// Nothing to run; store the empty gateway so Apply can populate
		// it later without a nil check.
		c.g = g
		return nil
	}
	c.g = g
	started := make(chan struct{})
	c.started = started
	go func() {
		close(started)
		_ = g.Start(context.Background())
	}()
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

// Shutdown stops the current Gateway (best-effort) so callers can
// clean up without leaking goroutines.
func (c *Controller) Shutdown(ctx context.Context) {
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
// directly without a second mapping layer.
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

	built, err := cli.BuildGateway(cli.BuildGatewayDeps{Config: *c.cfg})
	if err != nil {
		return api.ApplyResult{OK: false, TookMS: time.Since(start).Milliseconds()},
			fmt.Errorf("rebuild: %w", err)
	}

	c.mu.Lock()
	c.g = built
	c.mu.Unlock()

	names := built.Names()
	if len(names) > 0 {
		started := make(chan struct{})
		go func() {
			close(started)
			_ = built.Start(context.Background())
		}()
		<-started
	}

	return api.ApplyResult{
		OK:        true,
		Restarted: names,
		Errors:    map[string]string{},
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
