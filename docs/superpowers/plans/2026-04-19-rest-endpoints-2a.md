# REST Endpoints (Stage 2a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the REST server with the four platform management endpoints (`schema`, `reveal`, `test`, `apply`), harden `/api/config` for secrets, and make `hermind web` manage a live `gateway.Gateway` subsystem that Apply can restart in-process.

**Architecture:** A new `cli/gatewayctl.Controller` owns the running `gateway.Gateway` (plus its cancel fn and a serializing mutex); `hermind web` boots the controller next to the REST server. `api.ServerOpts.Controller` is a new interface the REST handlers call into — when nil (unit-test path), the four platform endpoints return 503. `gateway.Gateway.Stop` and a new `cli.BuildGateway` helper make the restart cycle clean. Per-descriptor `Test` closures stay nil in this plan; `POST /api/platforms/{key}/test` returns 501 until Stage 2b populates them.

**Tech Stack:** Go 1.22, `testing` + `net/http/httptest` for HTTP handler tests, `sync.Mutex` for apply serialization. No new third-party deps.

**Scope cut from spec §5–§7:**
- Per-type `descriptor.Test` closures → Stage 2b plan.
- Frontend consumption of the new endpoints → Stage 3 onwards.

**Source of truth:** `docs/superpowers/specs/2026-04-19-web-im-config-design.md` §§4, 5, 6, 7. Anything this plan deviates from is flagged inline.

---

## File Structure

**Create:**

- `cli/gateway_build.go` — `BuildGateway(ctx, cfg, primary, aux, storage, tools)` extracted from `runGateway`, returning a registered-but-not-yet-started `*gateway.Gateway`. Tested by `cli/gateway_build_test.go`.
- `cli/gatewayctl/controller.go` — `Controller` struct managing the live Gateway, with `Apply(ctx)` and `TestPlatform(ctx, key)` methods. Tested by `cli/gatewayctl/controller_test.go`.
- `api/handlers_platforms.go` — four handlers: `handlePlatformsSchema`, `handlePlatformReveal`, `handlePlatformTest`, `handlePlatformsApply`. Tested by `api/handlers_platforms_test.go`.

**Modify:**

- `gateway/gateway.go` — add `Stop(ctx)` method + internal cancel tracking.
- `cli/gateway.go` — `runGateway` now calls `BuildGateway`; behavior unchanged.
- `cli/web.go` — boot a `gatewayctl.Controller` next to the REST server; wire via `ServerOpts.Controller`.
- `api/server.go` — declare `GatewayController` interface, extend `ServerOpts`, mount 4 routes.
- `api/dto.go` — add `PlatformsSchemaResponse`, `RevealRequest`, `RevealResponse`, `PlatformTestResponse`, `ApplyResponse`.
- `api/handlers_config.go` — redact secrets on GET, preserve-on-empty on PUT.

**Untouched:**

- All 19 `descriptor_*.go` files (Test closures come in Stage 2b).
- `cli/gateway_test.go`, `gateway/platforms/*` tests.
- Any frontend files.

---

## Task 1: `gateway.Gateway.Stop(ctx)`

The current `Start(ctx)` blocks until the caller cancels its own ctx; there is no in-band way to ask the Gateway itself to stop. `Controller.Apply` needs one.

**Files:**
- Modify: `gateway/gateway.go`
- Test: `gateway/gateway_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `gateway/gateway_test.go`:

```go
func TestGateway_StopUnblocksStart(t *testing.T) {
	g := gateway.NewGateway(config.Config{}, nil, nil, nil, nil)
	g.Register(newStopTestPlatform("p1"))

	startErr := make(chan error, 1)
	go func() {
		startErr <- g.Start(context.Background())
	}()

	// Give Start a moment to enter its wait loop.
	time.Sleep(20 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.Stop(stopCtx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	select {
	case err := <-startErr:
		if err != nil && err != context.Canceled {
			t.Errorf("Start returned %v, want nil or context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of Stop")
	}
}

// stopTestPlatform is a minimal Platform that blocks in Run until ctx
// is cancelled, then returns ctx.Err().
type stopTestPlatform struct{ name string }

func newStopTestPlatform(name string) *stopTestPlatform { return &stopTestPlatform{name: name} }
func (p *stopTestPlatform) Name() string                 { return p.name }
func (p *stopTestPlatform) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (p *stopTestPlatform) SendReply(_ context.Context, _ gateway.OutgoingMessage) error { return nil }
```

Also make sure the file imports `"context"`, `"testing"`, `"time"`, and `"github.com/odysseythink/hermind/gateway"`. If it already has them, skip.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./gateway/ -run TestGateway_StopUnblocksStart -v`

Expected: FAIL — `g.Stop undefined`.

- [ ] **Step 3: Add `Stop` to `gateway.Gateway`**

In `gateway/gateway.go`, add a `stopFn context.CancelFunc` field to the `Gateway` struct and mutate `Start` to install it. Full change:

Inside the `Gateway` struct definition (next to `hooks`/`channels`):

```go
	// stopFn cancels the context passed into Start; nil when not running.
	stopFn   context.CancelFunc
	stopOnce sync.Once
	stopMu   sync.Mutex
```

Replace the first 5 lines of `Start`:

```go
func (g *Gateway) Start(ctx context.Context) error {
	if len(g.platforms) == 0 {
		return fmt.Errorf("gateway: no platforms registered")
	}
```

with:

```go
func (g *Gateway) Start(ctx context.Context) error {
	if len(g.platforms) == 0 {
		return fmt.Errorf("gateway: no platforms registered")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g.stopMu.Lock()
	g.stopFn = cancel
	g.stopOnce = sync.Once{}
	g.stopMu.Unlock()
```

Add a new method at the end of the file (before the final closing brace of the package, i.e. at file end):

```go
// Stop asks the Gateway to cancel its Start loop. Safe to call from
// any goroutine and idempotent within a single Start cycle; a second
// Stop during the same run is a no-op. Respects the caller's ctx for
// its own deadline but does not wait for Start to return — callers
// that care should block on the channel they already use with Start.
func (g *Gateway) Stop(ctx context.Context) error {
	g.stopMu.Lock()
	cancel := g.stopFn
	once := &g.stopOnce
	g.stopMu.Unlock()
	if cancel == nil {
		return nil
	}
	once.Do(cancel)
	return ctx.Err()
}
```

- [ ] **Step 4: Verify the test now passes**

Run: `go test ./gateway/ -run TestGateway_StopUnblocksStart -v`

Expected: PASS.

- [ ] **Step 5: Full gateway package tests still pass**

Run: `go test ./gateway/...`

Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add gateway/gateway.go gateway/gateway_test.go
git commit -m "$(cat <<'EOF'
feat(gateway): add Gateway.Stop for in-process shutdown

Start now installs an internal cancel func that Stop triggers via a
sync.Once. Callers that want to restart the subsystem (the upcoming
web apply endpoint) no longer need to cancel an outer ctx they may
not own.
EOF
)"
```

---

## Task 2: Extract `BuildGateway` helper

The ~40 lines in `runGateway` that wire `gateway.NewGateway` + tracing + metrics + register-platforms belong in a reusable function. `Controller.Apply` will call the same function.

**Files:**
- Create: `cli/gateway_build.go`
- Modify: `cli/gateway.go` (`runGateway` body only)
- Test: `cli/gateway_build_test.go`

- [ ] **Step 1: Write the failing test**

Create `cli/gateway_build_test.go`:

```go
package cli

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestBuildGateway_EmptyConfigBuildsEmptyGateway(t *testing.T) {
	cfg := config.Config{}
	g, err := BuildGateway(BuildGatewayDeps{Config: cfg})
	if err != nil {
		t.Fatalf("BuildGateway returned error: %v", err)
	}
	if g == nil {
		t.Fatal("BuildGateway returned nil")
	}
}

func TestBuildGateway_RegistersEnabledPlatforms(t *testing.T) {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {Enabled: true, Type: "telegram", Options: map[string]string{"token": "t"}},
		"off":     {Enabled: false, Type: "telegram", Options: map[string]string{"token": "t"}},
	}
	g, err := BuildGateway(BuildGatewayDeps{Config: cfg})
	if err != nil {
		t.Fatalf("BuildGateway: %v", err)
	}
	names := g.Names()
	if len(names) != 1 || names[0] != "telegram" {
		t.Errorf("registered platforms = %v, want [telegram]", names)
	}
}

func TestBuildGateway_UnknownTypeReturnsError(t *testing.T) {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"bad": {Enabled: true, Type: "does-not-exist"},
	}
	if _, err := BuildGateway(BuildGatewayDeps{Config: cfg}); err == nil {
		t.Fatal("BuildGateway(unknown type) returned nil error")
	}
}
```

This test references `g.Names()` — that helper doesn't exist yet; it will be added on `gateway.Gateway` in Step 3 as part of this task (trivial and reusable elsewhere).

- [ ] **Step 2: Run the test — expect compile failure**

Run: `go test ./cli/ -run TestBuildGateway -v`

Expected: FAIL — `undefined: BuildGateway`, `undefined: BuildGatewayDeps`, `g.Names undefined`.

- [ ] **Step 3: Add `Names()` to `gateway.Gateway`**

In `gateway/gateway.go`, next to the existing `Register` method, add:

```go
// Names returns the sorted list of registered platform names. Used by
// tests and by Controller for the "restarted" apply response.
func (g *Gateway) Names() []string {
	out := make([]string, 0, len(g.platforms))
	for name := range g.platforms {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
```

Ensure `"sort"` is in the imports.

- [ ] **Step 4: Create `cli/gateway_build.go`**

Exact content:

```go
package cli

import (
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/storage"
	"github.com/odysseythink/hermind/tool"
)

// BuildGatewayDeps bundles everything BuildGateway needs.
// Only Config is required; the rest may be zero/nil.
type BuildGatewayDeps struct {
	Config   config.Config
	Primary  provider.Provider
	Aux      provider.Provider
	Storage  storage.Storage
	Tools    *tool.Registry
}

// BuildGateway constructs a gateway.Gateway with all Enabled platforms
// from deps.Config.Gateway.Platforms already registered. The returned
// Gateway is not yet Started — callers control its lifecycle.
//
// Unknown platform types are an error (no silent skip). Disabled
// entries are silently skipped.
func BuildGateway(deps BuildGatewayDeps) (*gateway.Gateway, error) {
	g := gateway.NewGateway(deps.Config, deps.Primary, deps.Aux, deps.Storage, deps.Tools)
	for name, pc := range deps.Config.Gateway.Platforms {
		if !pc.Enabled {
			continue
		}
		plat, err := buildPlatform(name, pc)
		if err != nil {
			return nil, fmt.Errorf("gateway platform %q: %w", name, err)
		}
		g.Register(plat)
	}
	return g, nil
}
```

- [ ] **Step 5: Use `BuildGateway` from `runGateway`**

In `cli/gateway.go`, replace lines 116–126 (the block that reads `for name, pc := range app.Config.Gateway.Platforms { ... }` and below, where each platform is built and registered) with:

```go
	built, err := BuildGateway(BuildGatewayDeps{
		Config:  *app.Config,
		Primary: primary,
		Aux:     aux,
		Storage: app.Storage,
		Tools:   reg,
	})
	if err != nil {
		return err
	}
	// Copy the registered platforms onto the pre-existing g instance so
	// tracer/metrics wiring done above is preserved. Simplest path: drop
	// the pre-built g and reuse built directly.
	g = built
```

Keep everything above this block (tracing setup, metrics setup on `g`) intact, but **move** those setup calls to run AFTER `built` is assigned, operating on `built` instead of the earlier `g`. Full replacement of the refactored portion (replace lines 72–126, inclusive):

```go
	built, err := BuildGateway(BuildGatewayDeps{
		Config:  *app.Config,
		Primary: primary,
		Aux:     aux,
		Storage: app.Storage,
		Tools:   reg,
	})
	if err != nil {
		return err
	}
	g := built

	// Optional tracing.
	if app.Config.Tracing.Enabled {
		var w *os.File = os.Stderr
		if path := app.Config.Tracing.File; path != "" {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gateway: tracing: %v (falling back to stderr)\n", err)
			} else {
				w = f
				defer f.Close()
			}
		}
		exporter := tracing.NewJSONLinesExporter(w)
		tracer := tracing.NewTracer(exporter)
		g.SetTracer(tracer)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = tracer.Shutdown(shutdownCtx)
		}()
	}

	// Optional /metrics HTTP server.
	var metricsSrv *http.Server
	if addr := app.Config.Metrics.Addr; addr != "" {
		metricsReg := metrics.NewRegistry()
		g.SetMetrics(metricsReg)
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsReg)
		metricsSrv = &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "gateway: metrics server: %v\n", err)
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = metricsSrv.Shutdown(shutdownCtx)
		}()
	}
```

The subsequent signal/run block (lines 127+) stays unchanged; it already calls `g.Start(runCtx)`.

**Verify** after the edit that `cli/gateway.go` does NOT contain the line `for name, pc := range app.Config.Gateway.Platforms {` any more — the old registration loop is now inside `BuildGateway`.

- [ ] **Step 6: Run the new tests**

Run: `go test ./cli/ -run 'TestBuildGateway|TestBuildPlatform' -v`

Expected: all PASS (3 new + 4 existing TestBuildPlatform* from Stage 1).

- [ ] **Step 7: Run full CLI + gateway tests**

Run: `go test ./cli/... ./gateway/... ./gateway/platforms/...`

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add cli/gateway_build.go cli/gateway_build_test.go cli/gateway.go gateway/gateway.go
git commit -m "$(cat <<'EOF'
refactor(cli): extract BuildGateway + Gateway.Names helper

BuildGateway factors out the construct-and-register logic from
runGateway so the upcoming controller can call it directly. Adds
Gateway.Names() for tests + the apply response listing.
EOF
)"
```

---

## Task 3: `api.GatewayController` interface + sentinel errors + DTOs

*(Task 3 and Task 4 are swapped from an earlier draft — the API-package sentinels must exist before the controller can alias them.)*

The REST handlers should depend on an interface, not the concrete `gatewayctl.Controller`, so API tests can stub it. The shared sentinel errors (`ErrApplyInProgress`, `ErrTestNotImplemented`, `ErrUnknownPlatformKey`) also live here so both halves of the code can `errors.Is` them.

**Files:**
- Modify: `api/server.go` (new interface + ServerOpts field + sentinel errors)
- Modify: `api/dto.go` (four new DTOs)
- Test: `api/server_test.go` (append a smoke test that a nil Controller doesn't break existing behavior)

- [ ] **Step 1: Write the failing test**

Append to `api/server_test.go`:

```go
func TestNewServer_ControllerOptional(t *testing.T) {
	cfg := &config.Config{}
	srv, err := api.NewServer(&api.ServerOpts{
		Config: cfg,
		Token:  "test-token",
		// Controller: nil — acceptable, the platform endpoints will 503.
	})
	if err != nil {
		t.Fatalf("NewServer with nil Controller: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}
```

(Ensure the test file imports `"github.com/odysseythink/hermind/api"` and `"github.com/odysseythink/hermind/config"`. If it already does, skip.)

- [ ] **Step 2: Run the test — expect it to PASS on current code (the field doesn't exist yet, so the struct literal doesn't reference it)**

Run: `go test ./api/ -run TestNewServer_ControllerOptional -v`

Expected: PASS. We run it just to confirm baseline; the real check is that this test still passes after we add the Controller field.

- [ ] **Step 3: Add the `GatewayController` interface + sentinels to `api/server.go`**

At the top of `api/server.go` (just after the imports block, before `type ServerOpts struct`), add:

```go
// GatewayController is the subset of gatewayctl.Controller that the
// REST layer consumes. Keeping it as an interface avoids a cyclic
// import and lets handler tests stub it.
type GatewayController interface {
	// Apply performs a stop-rebuild-start cycle on the underlying
	// gateway subsystem. Returns ErrApplyInProgress if an Apply is
	// already running.
	Apply(ctx context.Context) (ApplyResult, error)

	// TestPlatform runs the platform's descriptor.Test for key.
	// Errors are surfaced verbatim; callers inspect ok/error for
	// user-facing mapping.
	TestPlatform(ctx context.Context, key string) error
}

// Sentinel errors shared between the API handlers and the controller
// implementation (cli/gatewayctl aliases these). Handlers use
// errors.Is against them regardless of which side returned the error.
var (
	ErrApplyInProgress    = errors.New("apply already in progress")
	ErrTestNotImplemented = errors.New("test not implemented for this platform type")
	ErrUnknownPlatformKey = errors.New("unknown platform key")
)
```

Also add `"context"` and `"errors"` to the imports if they are not already present.

Add a new field to `ServerOpts`:

```go
	// Controller manages the gateway lifecycle. nil means the four
	// /api/platforms/* endpoints return 503 Service Unavailable.
	Controller GatewayController
```

- [ ] **Step 4: Add the DTOs to `api/dto.go`**

Append:

```go
// SchemaFieldDTO describes one field of a platform descriptor.
type SchemaFieldDTO struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Help     string   `json:"help,omitempty"`
	Kind     string   `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Default  any      `json:"default,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// SchemaDescriptorDTO is one descriptor in the schema response.
type SchemaDescriptorDTO struct {
	Type        string           `json:"type"`
	DisplayName string           `json:"display_name"`
	Summary     string           `json:"summary,omitempty"`
	Fields      []SchemaFieldDTO `json:"fields"`
}

// PlatformsSchemaResponse is the payload for GET /api/platforms/schema.
type PlatformsSchemaResponse struct {
	Descriptors []SchemaDescriptorDTO `json:"descriptors"`
}

// RevealRequest is the body of POST /api/platforms/{key}/reveal.
type RevealRequest struct {
	Field string `json:"field"`
}

// RevealResponse is the success payload for reveal.
type RevealResponse struct {
	Value string `json:"value"`
}

// ErrorResponse is the generic error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// PlatformTestResponse is the payload for POST /api/platforms/{key}/test.
// ok=true on success; on failure, both ok=false and error are set.
type PlatformTestResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ApplyResult is the payload for POST /api/platforms/apply and also
// the return type of the GatewayController.Apply method. Shared so
// the controller can hand a value straight to the handler without an
// intermediate mapping.
type ApplyResult struct {
	OK        bool              `json:"ok"`
	Restarted []string          `json:"restarted,omitempty"`
	Errors    map[string]string `json:"errors,omitempty"`
	TookMS    int64             `json:"took_ms"`
	Error     string            `json:"error,omitempty"` // only on ok=false
}
```

- [ ] **Step 5: Run the smoke test and confirm it still PASSes**

Run: `go test ./api/ -run TestNewServer_ControllerOptional -v`

Expected: PASS — the field is optional (not required by NewServer validation).

- [ ] **Step 6: Run full api package tests**

Run: `go test ./api/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/server.go api/dto.go api/server_test.go
git commit -m "$(cat <<'EOF'
feat(api): add GatewayController interface + DTOs for platform endpoints

Interface keeps api/ free of cli/gatewayctl imports. Shared sentinels
(ErrApplyInProgress, ErrTestNotImplemented, ErrUnknownPlatformKey)
are declared here so both sides of the interface can errors.Is
identical values. Upcoming handlers will 503 when Controller is nil.
EOF
)"
```

---

## Task 4: `cli/gatewayctl.Controller`

Holds the live Gateway, a cancel fn for its Start goroutine, an apply mutex, and the inputs needed to rebuild on Apply. Aliases the sentinel errors declared in Task 3 so a `gatewayctl.ErrApplyInProgress` value is the same identity as `api.ErrApplyInProgress`.

**Files:**
- Create: `cli/gatewayctl/controller.go`
- Create: `cli/gatewayctl/controller_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cli/gatewayctl/controller_test.go`:

```go
package gatewayctl_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/cli/gatewayctl"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
)

func TestController_ApplyRestartsGateway(t *testing.T) {
	cfg := stubCfg("stub_a")
	ctrl := gatewayctl.New(&cfg)

	if err := ctrl.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ctrl.Shutdown(context.Background())

	names1 := ctrl.Running()
	if len(names1) != 1 || names1[0] != "stub_a" {
		t.Errorf("before apply: running = %v", names1)
	}

	// Mutate the config and apply.
	cfg.Gateway.Platforms["stub_b"] = config.PlatformConfig{
		Enabled: true, Type: "stub",
		Options: map[string]string{"name": "stub_b"},
	}

	res, err := ctrl.Apply(context.Background())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var _ api.ApplyResult = res // compile-time confirmation of return type
	if !res.OK {
		t.Errorf("res.OK = false, errors = %v", res.Errors)
	}
	names2 := ctrl.Running()
	if len(names2) != 2 {
		t.Errorf("after apply: running = %v, want 2 names", names2)
	}
}

func TestController_ApplyConcurrent409(t *testing.T) {
	cfg := stubCfg("stub_a")
	ctrl := gatewayctl.New(&cfg)
	if err := ctrl.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ctrl.Shutdown(context.Background())

	// Install a slow descriptor so the first Apply holds the mutex
	// long enough for the second to contend. Left registered for the
	// rest of the test binary's life — no other test uses this type.
	platforms.Register(platforms.Descriptor{
		Type:        "stub_slow",
		DisplayName: "Stub (slow build)",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			time.Sleep(150 * time.Millisecond)
			return newStubPlatform(opts["name"]), nil
		},
	})

	cfg.Gateway.Platforms["slow"] = config.PlatformConfig{
		Enabled: true, Type: "stub_slow",
		Options: map[string]string{"name": "slow"},
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = ctrl.Apply(context.Background())
		}()
	}
	wg.Wait()

	okCount, conflictCount := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			okCount++
		case errors.Is(err, gatewayctl.ErrApplyInProgress):
			conflictCount++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Errorf("got ok=%d conflict=%d, want 1+1", okCount, conflictCount)
	}
}

func TestController_TestPlatform(t *testing.T) {
	cfg := stubCfg("stub_a")
	ctrl := gatewayctl.New(&cfg)

	// descriptor "stub" has no Test func; expect ErrTestNotImplemented.
	if err := ctrl.TestPlatform(context.Background(), "stub_a"); !errors.Is(err, gatewayctl.ErrTestNotImplemented) {
		t.Errorf("TestPlatform(stub without Test): got %v, want ErrTestNotImplemented", err)
	}

	// Unknown key: expect ErrUnknownKey.
	if err := ctrl.TestPlatform(context.Background(), "no_such"); !errors.Is(err, gatewayctl.ErrUnknownKey) {
		t.Errorf("TestPlatform(unknown key): got %v, want ErrUnknownKey", err)
	}
}

func TestController_TestPlatformCallsDescriptorTest(t *testing.T) {
	// Register a stub_testable descriptor whose Test returns a sentinel
	// error; confirm it propagates.
	sentinel := errors.New("probe failed")
	platforms.Register(platforms.Descriptor{
		Type:        "stub_testable",
		DisplayName: "Stub (with Test)",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return newStubPlatform(opts["name"]), nil
		},
		Test: func(_ context.Context, _ map[string]string) error {
			return sentinel
		},
	})

	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"probed": {Enabled: true, Type: "stub_testable", Options: map[string]string{"name": "probed"}},
	}
	ctrl := gatewayctl.New(&cfg)

	if err := ctrl.TestPlatform(context.Background(), "probed"); !errors.Is(err, sentinel) {
		t.Errorf("TestPlatform: got %v, want %v", err, sentinel)
	}
}

// stubCfg returns a config with one "stub" platform under the given key.
func stubCfg(key string) config.Config {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		key: {Enabled: true, Type: "stub", Options: map[string]string{"name": key}},
	}
	return cfg
}
```

Also create the shared stub platform + `init` that registers a "stub" descriptor, because tests need it. File `cli/gatewayctl/stub_test.go`:

```go
package gatewayctl_test

import (
	"context"

	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
)

func init() {
	platforms.Register(platforms.Descriptor{
		Type:        "stub",
		DisplayName: "Stub (test-only)",
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return newStubPlatform(opts["name"]), nil
		},
	})
}

type stubPlatform struct{ name string }

func newStubPlatform(name string) *stubPlatform { return &stubPlatform{name: name} }
func (s *stubPlatform) Name() string             { return s.name }
func (s *stubPlatform) Run(ctx context.Context, _ gateway.MessageHandler) error {
	<-ctx.Done()
	return ctx.Err()
}
func (s *stubPlatform) SendReply(_ context.Context, _ gateway.OutgoingMessage) error { return nil }
```

- [ ] **Step 2: Run the tests — expect compile failure**

Run: `go test ./cli/gatewayctl/... -v`

Expected: FAIL — `undefined: gatewayctl.New`, `ErrApplyInProgress`, `ErrTestNotImplemented`, `ErrUnknownKey`.

- [ ] **Step 3: Implement `controller.go`**

Create `cli/gatewayctl/controller.go`:

```go
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

	mu      sync.Mutex      // guards g + startCancel
	g       *gateway.Gateway
	started chan struct{}   // closed after Start returned; nil when not running

	applyMu sync.Mutex
}

// New returns a Controller bound to the given config pointer. The
// pointer is used live — callers must not swap it out and must
// serialize mutations through PUT /api/config.
func New(cfg *config.Config) *Controller {
	return &Controller{cfg: cfg}
}

// Start builds and runs the initial Gateway in a background goroutine.
// Returns once Start has entered its wait loop (best-effort: we wait
// for a tiny amount for platforms to register into the Gateway).
// Start is idempotent-ish: calling it twice on an already-running
// controller returns an error.
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

// Shutdown stops the current Gateway (best-effort, ignoring errors) so
// callers can clean up without leaking goroutines.
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./cli/gatewayctl/... -v`

Expected: PASS for all four tests.

- [ ] **Step 5: Run broader tests to ensure no regressions**

Run: `go test ./cli/... ./gateway/... ./gateway/platforms/...`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/gatewayctl/
git commit -m "$(cat <<'EOF'
feat(cli/gatewayctl): Controller owning gateway lifecycle

New package holds the live Gateway on behalf of hermind web. Apply
performs a serialized stop-rebuild-start cycle (ErrApplyInProgress on
concurrent call); TestPlatform delegates to the descriptor's Test
closure (ErrTestNotImplemented until Stage 2b populates them).
EOF
)"
```

---


## Task 5: Wire `Controller` into `hermind web`

`cli/web.go` today only launches the REST server. It now also starts a `gatewayctl.Controller` and hands it to the server via `ServerOpts.Controller`.

**Files:**
- Modify: `cli/web.go`
- Test: `cli/web_test.go` (append)

- [ ] **Step 1: Append a smoke test**

Append to `cli/web_test.go`:

```go
func TestWebCmd_StartsControllerWithEmptyConfig(t *testing.T) {
	// A config with no gateway platforms should still let the web
	// command boot the controller and the HTTP server. This test
	// binds ":0" and exits via --exit-after so the command returns.
	cfg := &config.Config{}
	app := &App{Config: cfg, ConfigPath: ""}
	cmd := newWebCmd(app)
	cmd.SetArgs([]string{"--addr=127.0.0.1:0", "--no-browser", "--exit-after=100ms"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("web cmd exited with: %v", err)
	}
}
```

If the test file already exists, just append the new test; otherwise create it. Ensure imports include `testing`, `config`, and anything needed.

- [ ] **Step 2: Run the test to confirm baseline**

Run: `go test ./cli/ -run TestWebCmd_StartsControllerWithEmptyConfig -v`

Expected: PASS (today the command already boots an empty config fine). We keep this test after the modification to prevent regressions.

- [ ] **Step 3: Modify `cli/web.go` to start the controller**

In the `RunE` block of `newWebCmd`, between the `ensureStorage` call and the `api.GenerateToken` call, insert:

```go
			ctrl := gatewayctl.New(app.Config)
			if err := ctrl.Start(cmd.Context()); err != nil {
				return fmt.Errorf("web: start gateway controller: %w", err)
			}
			defer func() {
				shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
				defer c2()
				ctrl.Shutdown(shutCtx)
			}()
```

Add `"github.com/odysseythink/hermind/cli/gatewayctl"` to the imports.

In the `api.NewServer` call, add:

```go
				Controller: ctrl,
```

as an additional field — placed under `Streams: streams,`. The whole `NewServer` call becomes:

```go
			srv, err := api.NewServer(&api.ServerOpts{
				Config:     app.Config,
				ConfigPath: app.ConfigPath,
				Storage:    app.Storage,
				Token:      token,
				Version:    Version,
				Streams:    streams,
				Controller: ctrl,
			})
```

- [ ] **Step 4: Run the smoke test and verify it still PASSes**

Run: `go test ./cli/ -run TestWebCmd_StartsControllerWithEmptyConfig -v`

Expected: PASS.

- [ ] **Step 5: Run full CLI tests**

Run: `go test ./cli/...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/web.go cli/web_test.go
git commit -m "$(cat <<'EOF'
feat(cli/web): boot gatewayctl.Controller alongside REST server

hermind web now owns a live gateway alongside the REST API so the
upcoming /api/platforms/apply endpoint can stop-and-restart it in
process.
EOF
)"
```

---

## Task 6: GET `/api/config` redacts secrets

Walk `gateway.platforms` on output; replace every FieldSecret value with `""`. Non-gateway config is untouched.

**Files:**
- Modify: `api/handlers_config.go`
- Test: `api/handlers_config_test.go` (append)

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_test.go`:

```go
func TestHandleConfigGet_RedactsSecretFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "super-secret-123"},
		},
	}
	srv := newTestServer(t, cfg)

	req := httptest.NewRequest("GET", "/api/config", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	gw := body.Config["gateway"].(map[string]any)
	platforms := gw["platforms"].(map[string]any)
	inst := platforms["tg_main"].(map[string]any)
	options := inst["options"].(map[string]any)
	if got := options["token"]; got != "" {
		t.Errorf("options.token = %q, want \"\" (redacted)", got)
	}
}

// newTestServer is a small helper if one doesn't already exist — add
// it alongside the test. If handlers_config_test.go already has a
// helper by a similar name, just use that instead.
func newTestServer(t *testing.T, cfg *config.Config) *api.Server {
	t.Helper()
	srv, err := api.NewServer(&api.ServerOpts{
		Config: cfg,
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}
```

Ensure imports: `encoding/json`, `net/http`, `net/http/httptest`, `testing`, `api`, `config`. Reuse an existing helper if the test file already provides one — don't duplicate.

- [ ] **Step 2: Run the test — expect it to FAIL (token currently returned verbatim)**

Run: `go test ./api/ -run TestHandleConfigGet_RedactsSecretFields -v`

Expected: FAIL — `options.token = "super-secret-123", want ""`.

- [ ] **Step 3: Implement redaction**

In `api/handlers_config.go`, replace the `handleConfigGet` function body with:

```go
func (s *Server) handleConfigGet(w http.ResponseWriter, _ *http.Request) {
	data, err := yaml.Marshal(s.opts.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redactSecrets(m)
	writeJSON(w, ConfigResponse{Config: m})
}

// redactSecrets walks m["gateway"]["platforms"][*]["options"], consults
// the platform registry for each entry's Type, and blanks every field
// whose Kind is FieldSecret. Silently ignores unknown types or missing
// sections — we're redacting defensively, not validating.
func redactSecrets(m map[string]any) {
	gw, _ := m["gateway"].(map[string]any)
	plats, _ := gw["platforms"].(map[string]any)
	for _, raw := range plats {
		inst, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := inst["type"].(string)
		if typ == "" {
			continue
		}
		d, ok := platforms.Get(typ)
		if !ok {
			continue
		}
		opts, _ := inst["options"].(map[string]any)
		if opts == nil {
			continue
		}
		for _, f := range d.Fields {
			if f.Kind == platforms.FieldSecret {
				if _, present := opts[f.Name]; present {
					opts[f.Name] = ""
				}
			}
		}
	}
}
```

Add `"github.com/odysseythink/hermind/gateway/platforms"` to the imports of `api/handlers_config.go`.

- [ ] **Step 4: Run the test**

Run: `go test ./api/ -run TestHandleConfigGet_RedactsSecretFields -v`

Expected: PASS.

- [ ] **Step 5: Run full api tests**

Run: `go test ./api/...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "$(cat <<'EOF'
feat(api/config): redact platform secret fields on GET /api/config

Consults the descriptor registry to find FieldSecret entries under
gateway.platforms.*.options and overwrites their values with "". The
UI relies on this + the Stage-2 empty-preserves-prior PUT to round-
trip secrets safely.
EOF
)"
```

---

## Task 7: PUT `/api/config` preserves unchanged secrets

If the incoming payload carries `""` for a FieldSecret value, keep whatever is already on disk for that key; otherwise overwrite with the incoming value.

**Files:**
- Modify: `api/handlers_config.go`
- Test: `api/handlers_config_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append to `api/handlers_config_test.go`:

```go
func TestHandleConfigPut_PreservesUnchangedSecret(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "live-token"},
		},
	}
	if err := config.SaveToPath(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	srv, err := api.NewServer(&api.ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// PUT the config back with token blanked.
	put := fmt.Sprintf(`{"config":{"gateway":{"platforms":{"tg_main":{"enabled":true,"type":"telegram","options":{"token":""}}}}}}`)
	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader(put))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := cfg.Gateway.Platforms["tg_main"].Options["token"]; got != "live-token" {
		t.Errorf("in-memory token = %q, want %q (preserved)", got, "live-token")
	}
}

func TestHandleConfigPut_OverwritesSecretWhenProvided(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "old-token"},
		},
	}
	if err := config.SaveToPath(path, cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	srv, err := api.NewServer(&api.ServerOpts{
		Config:     cfg,
		ConfigPath: path,
		Token:      "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	put := `{"config":{"gateway":{"platforms":{"tg_main":{"enabled":true,"type":"telegram","options":{"token":"new-token"}}}}}}`
	req := httptest.NewRequest("PUT", "/api/config", strings.NewReader(put))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if got := cfg.Gateway.Platforms["tg_main"].Options["token"]; got != "new-token" {
		t.Errorf("in-memory token = %q, want %q (overwritten)", got, "new-token")
	}
}
```

Imports needed: `"fmt"`, `"path/filepath"`, `"strings"`. Skip if already present.

- [ ] **Step 2: Run the tests — expect failures**

Run: `go test ./api/ -run 'TestHandleConfigPut_PreservesUnchangedSecret|TestHandleConfigPut_OverwritesSecretWhenProvided' -v`

Expected:
- Preserve test FAILs: the PUT today round-trips `""` to disk and memory, so `token = ""`, want `"live-token"`.
- Overwrite test PASSES today (no merge logic, straight overwrite), but verify — keep it in place as a regression guard once the preserve logic lands.

- [ ] **Step 3: Implement the merge logic**

In `api/handlers_config.go`, replace `handleConfigPut` with:

```go
func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	if s.opts.ConfigPath == "" {
		http.Error(w, "config write-back not configured", http.StatusNotImplemented)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	var req struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var updated config.Config
	if err := yaml.Unmarshal(req.Config, &updated); err != nil {
		http.Error(w, "invalid config payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	preserveSecrets(&updated, s.opts.Config)
	if err := config.SaveToPath(s.opts.ConfigPath, &updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	*s.opts.Config = updated
	writeJSON(w, OKResponse{OK: true})
}

// preserveSecrets copies every FieldSecret from current into updated
// whenever updated's value is "". Keys missing from current (new
// instances) are left with whatever the caller supplied.
func preserveSecrets(updated, current *config.Config) {
	for key, newPC := range updated.Gateway.Platforms {
		curPC, ok := current.Gateway.Platforms[key]
		if !ok {
			continue
		}
		d, ok := platforms.Get(newPC.Type)
		if !ok {
			continue
		}
		if newPC.Options == nil {
			newPC.Options = map[string]string{}
		}
		for _, f := range d.Fields {
			if f.Kind != platforms.FieldSecret {
				continue
			}
			if newPC.Options[f.Name] == "" {
				newPC.Options[f.Name] = curPC.Options[f.Name]
			}
		}
		updated.Gateway.Platforms[key] = newPC
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./api/ -run TestHandleConfigPut -v`

Expected: both new tests PASS, plus any pre-existing `TestHandleConfigPut_*` tests.

- [ ] **Step 5: Full api tests**

Run: `go test ./api/...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/handlers_config.go api/handlers_config_test.go
git commit -m "$(cat <<'EOF'
feat(api/config): preserve unchanged secrets on PUT /api/config

Empty-string FieldSecret values keep whatever is in the in-memory
config; non-empty values overwrite. Pairs with the redact-on-GET
behavior so the UI can round-trip a platform config without ever
seeing or having to re-paste the secret.
EOF
)"
```

---

## Task 8: GET `/api/platforms/schema`

Returns every registered descriptor shaped as `PlatformsSchemaResponse`. No controller dependency — reads the in-process registry.

**Files:**
- Create: `api/handlers_platforms.go`
- Modify: `api/server.go` (one route)
- Test: `api/handlers_platforms_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/handlers_platforms_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
)

func TestPlatformsSchema_ContainsAllRegisteredTypes(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.PlatformsSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Must include every production type.
	want := []string{
		"api_server", "acp", "webhook",
		"telegram", "discord", "discord_bot",
		"slack", "slack_events",
		"mattermost", "mattermost_bot",
		"feishu", "dingtalk", "wecom",
		"matrix", "signal", "whatsapp",
		"homeassistant", "email", "sms",
	}
	have := map[string]bool{}
	for _, d := range body.Descriptors {
		have[d.Type] = true
	}
	for _, t0 := range want {
		if !have[t0] {
			t.Errorf("missing descriptor: %q", t0)
		}
	}
}

func TestPlatformsSchema_TelegramFieldShape(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var body api.PlatformsSchemaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	var tg *api.SchemaDescriptorDTO
	for i := range body.Descriptors {
		if body.Descriptors[i].Type == "telegram" {
			tg = &body.Descriptors[i]
			break
		}
	}
	if tg == nil {
		t.Fatal("telegram descriptor not in response")
	}
	if len(tg.Fields) != 1 {
		t.Fatalf("telegram fields = %d, want 1", len(tg.Fields))
	}
	if tg.Fields[0].Kind != "secret" {
		t.Errorf("token kind = %q, want secret", tg.Fields[0].Kind)
	}
	if !tg.Fields[0].Required {
		t.Errorf("token.Required = false, want true")
	}
}

func TestPlatformsSchema_RequiresAuth(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
```

- [ ] **Step 2: Run the tests — expect compile/route failures**

Run: `go test ./api/ -run TestPlatformsSchema -v`

Expected: first two tests FAIL with 404 (route not registered); auth test PASSes (404 w/o auth is still denied by middleware; returns 401).

- [ ] **Step 3: Create `api/handlers_platforms.go`**

```go
package api

import (
	"net/http"

	"github.com/odysseythink/hermind/gateway/platforms"
)

func (s *Server) handlePlatformsSchema(w http.ResponseWriter, _ *http.Request) {
	all := platforms.All()
	out := PlatformsSchemaResponse{Descriptors: make([]SchemaDescriptorDTO, 0, len(all))}
	for _, d := range all {
		fields := make([]SchemaFieldDTO, 0, len(d.Fields))
		for _, f := range d.Fields {
			fields = append(fields, SchemaFieldDTO{
				Name:     f.Name,
				Label:    f.Label,
				Help:     f.Help,
				Kind:     f.Kind.String(),
				Required: f.Required,
				Default:  f.Default,
				Enum:     f.Enum,
			})
		}
		out.Descriptors = append(out.Descriptors, SchemaDescriptorDTO{
			Type:        d.Type,
			DisplayName: d.DisplayName,
			Summary:     d.Summary,
			Fields:      fields,
		})
	}
	writeJSON(w, out)
}
```

- [ ] **Step 4: Register the route in `api/server.go`**

Inside `buildRouter`'s `r.Route("/api", ...)` block, immediately after the existing `r.Get("/providers", s.handleProvidersList)` line, add:

```go
			r.Get("/platforms/schema", s.handlePlatformsSchema)
```

- [ ] **Step 5: Re-run the tests**

Run: `go test ./api/ -run TestPlatformsSchema -v`

Expected: all 3 PASS.

- [ ] **Step 6: Full api tests**

Run: `go test ./api/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_platforms.go api/server.go api/handlers_platforms_test.go
git commit -m "$(cat <<'EOF'
feat(api/platforms): GET /api/platforms/schema

Serializes every registered descriptor into JSON so the frontend can
render type-aware forms. No config touched; reads the in-process
registry. Bearer-auth required like all /api/* endpoints.
EOF
)"
```

---

## Task 9: POST `/api/platforms/{key}/reveal`

Validates key exists in `gateway.platforms`, field is `FieldSecret`, returns plaintext value.

**Files:**
- Modify: `api/handlers_platforms.go` (append handler)
- Modify: `api/server.go` (one route)
- Test: `api/handlers_platforms_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append to `api/handlers_platforms_test.go`:

```go
func TestPlatformReveal_ReturnsSecretValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true,
			Type:    "telegram",
			Options: map[string]string{"token": "live-token"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/tg_main/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.RevealResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if body.Value != "live-token" {
		t.Errorf("value = %q, want %q", body.Value, "live-token")
	}
}

func TestPlatformReveal_RejectsNonSecretField(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"slack_ops": {
			Enabled: true,
			Type:    "slack_events",
			Options: map[string]string{"addr": ":9000", "bot_token": "xoxb-y"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/slack_ops/reveal", strings.NewReader(`{"field":"addr"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPlatformReveal_404OnUnknownKey(t *testing.T) {
	cfg := &config.Config{}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/missing/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
```

Add `"strings"` to imports if not already present.

- [ ] **Step 2: Run the tests to confirm they fail**

Run: `go test ./api/ -run TestPlatformReveal -v`

Expected: all 3 FAIL with 404 (route missing).

- [ ] **Step 3: Append handler to `api/handlers_platforms.go`**

```go
func (s *Server) handlePlatformReveal(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	pc, ok := s.opts.Config.Gateway.Platforms[key]
	if !ok {
		writeJSONStatus(w, http.StatusNotFound, ErrorResponse{Error: "unknown platform key"})
		return
	}
	var req RevealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	d, ok := platforms.Get(pc.Type)
	if !ok {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "unknown platform type"})
		return
	}
	var fieldSpec *platforms.FieldSpec
	for i := range d.Fields {
		if d.Fields[i].Name == req.Field {
			fieldSpec = &d.Fields[i]
			break
		}
	}
	if fieldSpec == nil {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "no such field"})
		return
	}
	if fieldSpec.Kind != platforms.FieldSecret {
		writeJSONStatus(w, http.StatusBadRequest, ErrorResponse{Error: "field is not secret"})
		return
	}
	writeJSON(w, RevealResponse{Value: pc.Options[req.Field]})
}
```

Add `"encoding/json"` and `"github.com/go-chi/chi/v5"` to the imports of `handlers_platforms.go`.

Also create the tiny helper `writeJSONStatus` — add it to `api/server.go` just below the existing `writeJSON` helper:

```go
// writeJSONStatus is like writeJSON but sets a non-200 status code first.
func writeJSONStatus(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Register the route**

In `api/server.go`, just after the schema route, add:

```go
			r.Post("/platforms/{key}/reveal", s.handlePlatformReveal)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./api/ -run TestPlatformReveal -v`

Expected: all 3 PASS.

- [ ] **Step 6: Full api tests**

Run: `go test ./api/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_platforms.go api/handlers_platforms_test.go api/server.go
git commit -m "$(cat <<'EOF'
feat(api/platforms): POST /api/platforms/{key}/reveal

Returns the plaintext of one FieldSecret field for a saved platform
instance. 404 on unknown key, 400 on missing/non-secret field, 401 on
missing token. Reads in-memory config so users see the value they
just PUT (not the on-disk state).
EOF
)"
```

---

## Task 10: POST `/api/platforms/{key}/test`

Calls `Controller.TestPlatform`; 501 when controller is nil or the descriptor has no Test closure; always 200 with `{ok, error}` otherwise.

**Files:**
- Modify: `api/handlers_platforms.go` (append handler)
- Modify: `api/server.go` (one route)
- Test: `api/handlers_platforms_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append:

```go
type stubController struct {
	testErr    error
	applyCalls int
	applyRes   api.ApplyResult
	applyErr   error
}

func (s *stubController) TestPlatform(_ context.Context, _ string) error {
	return s.testErr
}
func (s *stubController) Apply(_ context.Context) (api.ApplyResult, error) {
	s.applyCalls++
	return s.applyRes, s.applyErr
}

func TestPlatformTest_NilControllerReturns503(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{Config: &config.Config{}, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestPlatformTest_NotImplementedReturns501(t *testing.T) {
	ctrl := &stubController{testErr: api.ErrTestNotImplemented}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
}

func TestPlatformTest_SuccessReturnsOK(t *testing.T) {
	ctrl := &stubController{testErr: nil}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.PlatformTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !body.OK {
		t.Errorf("body.OK = false, error = %q", body.Error)
	}
}

func TestPlatformTest_FailureReturnsOKFalse(t *testing.T) {
	ctrl := &stubController{testErr: errors.New("auth failed: bad token")}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.PlatformTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.OK {
		t.Error("body.OK = true, want false")
	}
	if !strings.Contains(body.Error, "auth failed") {
		t.Errorf("body.Error = %q, want substring 'auth failed'", body.Error)
	}
}
```

Add imports: `"context"`, `"errors"`. Skip if already there.

- [ ] **Step 2: Run the tests — expect failures**

Run: `go test ./api/ -run TestPlatformTest -v`

Expected: all 4 FAIL with 404 (route missing).

- [ ] **Step 3: Append handler**

In `api/handlers_platforms.go`:

```go
func (s *Server) handlePlatformTest(w http.ResponseWriter, r *http.Request) {
	if s.opts.Controller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			ErrorResponse{Error: "gateway controller not configured"})
		return
	}
	key := chi.URLParam(r, "key")
	err := s.opts.Controller.TestPlatform(r.Context(), key)
	switch {
	case err == nil:
		writeJSON(w, PlatformTestResponse{OK: true})
	case errors.Is(err, ErrTestNotImplemented):
		writeJSONStatus(w, http.StatusNotImplemented,
			ErrorResponse{Error: "test not implemented for this platform type"})
	case errors.Is(err, ErrUnknownPlatformKey):
		writeJSONStatus(w, http.StatusNotFound,
			ErrorResponse{Error: "unknown platform key"})
	default:
		writeJSON(w, PlatformTestResponse{OK: false, Error: err.Error()})
	}
}
```

Add `"errors"` to imports.

The sentinels used here (`ErrTestNotImplemented`, `ErrUnknownPlatformKey`) were already declared in Task 3 and aliased by `cli/gatewayctl` in Task 4, so `errors.Is` identifies the same value regardless of which side produced it. No further controller changes are needed.

- [ ] **Step 4: Register the route**

In `api/server.go`:

```go
			r.Post("/platforms/{key}/test", s.handlePlatformTest)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./api/ -run TestPlatformTest -v`

Expected: all 4 PASS.

- [ ] **Step 6: Full api + gatewayctl tests**

Run: `go test ./api/... ./cli/gatewayctl/...`

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_platforms.go api/handlers_platforms_test.go api/server.go
git commit -m "$(cat <<'EOF'
feat(api/platforms): POST /api/platforms/{key}/test

Delegates to Controller.TestPlatform. 503 when no controller is
configured (e.g., api.NewServer invoked without cli/web); 501 when the
descriptor has no Test closure (Stage-2a placeholder); 404 on unknown
key; otherwise 200 with {ok, error} body.
EOF
)"
```

---

## Task 11: POST `/api/platforms/apply`

Delegates to `Controller.Apply`; maps `ErrApplyInProgress` to 409; returns `ApplyResult` body on success.

**Files:**
- Modify: `api/handlers_platforms.go` (append handler)
- Modify: `api/server.go` (one route)
- Test: `api/handlers_platforms_test.go` (append)

- [ ] **Step 1: Write the failing tests**

Append:

```go
func TestPlatformsApply_NilControllerReturns503(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{Config: &config.Config{}, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestPlatformsApply_SuccessReturnsPayload(t *testing.T) {
	ctrl := &stubController{
		applyRes: api.ApplyResult{OK: true, Restarted: []string{"tg_main"}, TookMS: 42},
	}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.ApplyResult
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !body.OK || len(body.Restarted) != 1 || body.Restarted[0] != "tg_main" {
		t.Errorf("body = %+v", body)
	}
	if ctrl.applyCalls != 1 {
		t.Errorf("applyCalls = %d, want 1", ctrl.applyCalls)
	}
}

func TestPlatformsApply_ConcurrentReturns409(t *testing.T) {
	ctrl := &stubController{applyErr: api.ErrApplyInProgress}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestPlatformsApply_GenericErrorReturnsOKFalse(t *testing.T) {
	ctrl := &stubController{applyErr: errors.New("rebuild failed: config parse error")}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config:     &config.Config{},
		Token:      "test-token",
		Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.ApplyResult
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.OK {
		t.Error("body.OK = true, want false")
	}
	if !strings.Contains(body.Error, "rebuild failed") {
		t.Errorf("body.Error = %q", body.Error)
	}
}
```

- [ ] **Step 2: Run the tests — expect failures**

Run: `go test ./api/ -run TestPlatformsApply -v`

Expected: 4 FAIL with 404 or wrong status.

- [ ] **Step 3: Append handler**

In `api/handlers_platforms.go`:

```go
func (s *Server) handlePlatformsApply(w http.ResponseWriter, r *http.Request) {
	if s.opts.Controller == nil {
		writeJSONStatus(w, http.StatusServiceUnavailable,
			ErrorResponse{Error: "gateway controller not configured"})
		return
	}
	res, err := s.opts.Controller.Apply(r.Context())
	switch {
	case err == nil:
		writeJSON(w, res)
	case errors.Is(err, ErrApplyInProgress):
		writeJSONStatus(w, http.StatusConflict,
			ErrorResponse{Error: "apply already in progress"})
	default:
		// Controller returned a real error — surface it in ok=false shape.
		writeJSON(w, ApplyResult{OK: false, Error: err.Error(), TookMS: res.TookMS})
	}
}
```

`ErrApplyInProgress` was declared in Task 3 and already aliased by `cli/gatewayctl` in Task 4, so `errors.Is` on the handler side matches whether the error originated locally or from the controller.

- [ ] **Step 4: Register the route**

```go
			r.Post("/platforms/apply", s.handlePlatformsApply)
```

- [ ] **Step 5: Run the tests**

Run: `go test ./api/ -run TestPlatformsApply -v`

Expected: all 4 PASS.

- [ ] **Step 6: Full api + gatewayctl tests**

Run: `go test ./api/... ./cli/gatewayctl/...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_platforms.go api/handlers_platforms_test.go api/server.go
git commit -m "$(cat <<'EOF'
feat(api/platforms): POST /api/platforms/apply

Delegates to Controller.Apply. 503 when no controller; 409 on
concurrent apply via the shared ErrApplyInProgress sentinel; 200 with
ApplyResult on success or generic failure.
EOF
)"
```

---

## Task 12: End-to-end verification

No code changes; this is the "curl can drive the full flow" smoke test from spec §11.

**Files:** none modified.

- [ ] **Step 1: Build and launch**

```bash
(cd <worktree> && go build -o bin/hermind ./cmd/hermind && ./bin/hermind web --addr=127.0.0.1:0 --no-browser --exit-after=10m &)
```

Note the printed URL and token. For the rest of the steps, export them as shell variables:

```bash
export HERMIND_URL="http://127.0.0.1:<port>"
export HERMIND_TOK="<token>"
```

- [ ] **Step 2: Confirm schema returns all 19 types**

```bash
curl -s -H "Authorization: Bearer $HERMIND_TOK" $HERMIND_URL/api/platforms/schema | jq '.descriptors | length'
```

Expected: `19`.

- [ ] **Step 3: PUT a new telegram platform with an empty token**

```bash
curl -s -X PUT -H "Authorization: Bearer $HERMIND_TOK" -H "Content-Type: application/json" \
  -d '{"config":{"gateway":{"platforms":{"tg_test":{"enabled":true,"type":"telegram","options":{"token":"abc123"}}}}}}' \
  $HERMIND_URL/api/config
```

Expected: `{"ok":true}`.

- [ ] **Step 4: GET /api/config and confirm token is redacted**

```bash
curl -s -H "Authorization: Bearer $HERMIND_TOK" $HERMIND_URL/api/config | jq '.config.gateway.platforms.tg_test.options.token'
```

Expected: `""`.

- [ ] **Step 5: Reveal the token**

```bash
curl -s -X POST -H "Authorization: Bearer $HERMIND_TOK" -H "Content-Type: application/json" \
  -d '{"field":"token"}' $HERMIND_URL/api/platforms/tg_test/reveal
```

Expected: `{"value":"abc123"}`.

- [ ] **Step 6: Test returns 501 (no Test closure yet)**

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST -H "Authorization: Bearer $HERMIND_TOK" \
  $HERMIND_URL/api/platforms/tg_test/test
```

Expected: `501`.

- [ ] **Step 7: Apply succeeds**

```bash
curl -s -X POST -H "Authorization: Bearer $HERMIND_TOK" $HERMIND_URL/api/platforms/apply | jq .
```

Expected: `{"ok":true,"restarted":["telegram"],"errors":null,"took_ms":<N>}` — `restarted` lists the platform name reported by the Gateway, which for a single-instance config is the descriptor's own Name (`telegram`). If this feels confusing, follow-up Stage 2b can refine the `Restarted` field to return the user-chosen key instead; accepted for MVP.

- [ ] **Step 8: Second apply (concurrent) returns 409**

Send two applies in quick succession and verify one returns 409:

```bash
for i in 1 2; do curl -s -o /dev/null -w "%{http_code} " -X POST -H "Authorization: Bearer $HERMIND_TOK" $HERMIND_URL/api/platforms/apply & done
wait
echo
```

Expected: one `200`, one `409` (order may vary).

- [ ] **Step 9: Kill the test server**

```bash
kill %1 2>/dev/null || true
```

- [ ] **Step 10: Run the full test suite one more time**

```bash
(cd <worktree> && go test ./...)
```

Expected: all PASS (except pre-existing vet/test issues confirmed before Stage 1 — those are not ours to fix here).

---

## Rollback

`git reset --hard <commit-before-task-1>` on the feature branch. No migrations, no runtime state outside the config file you were testing with.

## Spec deltas worth flagging

1. **`BuildGateway` location:** spec §7 referenced "`cli/gateway.go::BuildGateway`"; we placed it in `cli/gateway_build.go` (same package `cli`) because the existing `gateway.go` is 200+ lines and the new helper is independently testable.
2. **Controller package:** spec didn't name one; this plan chose `cli/gatewayctl` so it sits under the CLI package tree that owns the lifecycle anyway.
3. **`Restarted` in ApplyResult:** today lists Gateway platform names (e.g. `telegram`), not user-chosen keys (`tg_main`). Flagged as Stage 2b or a follow-up; does not affect the contract shape.
4. **`descriptor.Test` closures:** all 19 stay `nil` through Stage 2a; `/test` endpoint returns 501 with a clear message. Populated in Stage 2b.
