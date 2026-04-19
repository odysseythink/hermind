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
	Config  config.Config
	Primary provider.Provider
	Aux     provider.Provider
	Storage storage.Storage
	Tools   *tool.Registry
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
