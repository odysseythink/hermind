package gatewayctl_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/cli/gatewayctl"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
)

// buildTestGateway builds a gateway from a config using only the
// test stubs defined in stub_test.go. Used by controller tests.
func buildTestGateway(cfg config.Config) (*gateway.Gateway, error) {
	g := gateway.NewGateway(cfg, nil, nil, nil, nil)
	for name, pc := range cfg.Gateway.Platforms {
		if !pc.Enabled {
			continue
		}
		plat, err := buildTestPlatform(name, pc)
		if err != nil {
			return nil, fmt.Errorf("gateway platform %q: %w", name, err)
		}
		g.Register(plat)
	}
	return g, nil
}

// buildTestPlatform mirrors the logic from cli.buildPlatform but uses
// only test stubs.
func buildTestPlatform(name string, pc config.PlatformConfig) (gateway.Platform, error) {
	desc, ok := platforms.Get(pc.Type)
	if !ok {
		return nil, fmt.Errorf("unknown platform type: %s", pc.Type)
	}
	return desc.Build(pc.Options)
}

func TestController_ApplyRestartsGateway(t *testing.T) {
	cfg := stubCfg("stub_a")
	ctrl := gatewayctl.New(&cfg, buildTestGateway)

	if err := ctrl.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer ctrl.Shutdown(context.Background())

	names1 := ctrl.Running()
	if len(names1) != 1 || names1[0] != "stub_a" {
		t.Errorf("before apply: running = %v", names1)
	}

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
	ctrl := gatewayctl.New(&cfg, buildTestGateway)
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
	ctrl := gatewayctl.New(&cfg, buildTestGateway)

	if err := ctrl.TestPlatform(context.Background(), "stub_a"); !errors.Is(err, gatewayctl.ErrTestNotImplemented) {
		t.Errorf("TestPlatform(stub without Test): got %v, want ErrTestNotImplemented", err)
	}

	if err := ctrl.TestPlatform(context.Background(), "no_such"); !errors.Is(err, gatewayctl.ErrUnknownKey) {
		t.Errorf("TestPlatform(unknown key): got %v, want ErrUnknownKey", err)
	}
}

func TestController_TestPlatformCallsDescriptorTest(t *testing.T) {
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
	ctrl := gatewayctl.New(&cfg, buildTestGateway)

	if err := ctrl.TestPlatform(context.Background(), "probed"); !errors.Is(err, sentinel) {
		t.Errorf("TestPlatform: got %v, want %v", err, sentinel)
	}
}

func stubCfg(key string) config.Config {
	cfg := config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		key: {Enabled: true, Type: "stub", Options: map[string]string{"name": key}},
	}
	return cfg
}
