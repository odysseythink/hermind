package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/gateway"
	"github.com/nousresearch/hermes-agent/gateway/platforms"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/factory"
	"github.com/nousresearch/hermes-agent/tool"
	"github.com/nousresearch/hermes-agent/tool/file"
	"github.com/nousresearch/hermes-agent/tool/terminal"
	"github.com/spf13/cobra"
)

// newGatewayCmd creates the "hermes gateway" subcommand.
func newGatewayCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "gateway",
		Short: "Run the multi-platform gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGateway(cmd.Context(), app)
		},
	}
}

func runGateway(ctx context.Context, app *App) error {
	if err := ensureStorage(app); err != nil {
		return err
	}

	primary, _, err := buildPrimaryProvider(app.Config)
	if err != nil {
		return err
	}
	var aux provider.Provider
	if app.Config.Auxiliary.APIKey != "" || app.Config.Auxiliary.Provider != "" {
		auxCfg := config.ProviderConfig{
			Provider: app.Config.Auxiliary.Provider,
			BaseURL:  app.Config.Auxiliary.BaseURL,
			APIKey:   app.Config.Auxiliary.APIKey,
			Model:    app.Config.Auxiliary.Model,
		}
		if auxCfg.Provider == "" {
			auxCfg.Provider = "anthropic"
		}
		if p, err := factory.New(auxCfg); err == nil {
			aux = p
		}
	}

	// File + local terminal only for the gateway to start with.
	reg := tool.NewRegistry()
	file.RegisterAll(reg)
	termBackend, err := terminal.New("local", terminal.Config{})
	if err == nil {
		terminal.RegisterShellExecute(reg, termBackend)
		defer termBackend.Close()
	}

	g := gateway.NewGateway(*app.Config, primary, aux, app.Storage, reg)

	for name, pc := range app.Config.Gateway.Platforms {
		if !pc.Enabled {
			continue
		}
		plat, err := buildPlatform(name, pc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gateway: skipping %s: %v\n", name, err)
			continue
		}
		g.Register(plat)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	return g.Start(runCtx)
}

// buildPlatform instantiates a platform adapter from its config entry.
func buildPlatform(name string, pc config.PlatformConfig) (gateway.Platform, error) {
	t := strings.ToLower(pc.Type)
	if t == "" {
		t = strings.ToLower(name)
	}
	switch t {
	case "api_server":
		return platforms.NewAPIServer(pc.Options["addr"]), nil
	case "webhook":
		return platforms.NewWebhook(pc.Options["url"], pc.Options["token"]), nil
	case "telegram":
		return platforms.NewTelegram(pc.Options["token"]), nil
	default:
		return nil, fmt.Errorf("unknown platform type %q", t)
	}
}
