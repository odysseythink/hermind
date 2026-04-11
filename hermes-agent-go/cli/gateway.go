package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"net/http"
	"time"

	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/gateway"
	"github.com/nousresearch/hermes-agent/gateway/platforms"
	"github.com/nousresearch/hermes-agent/metrics"
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
	case "acp":
		return platforms.NewACP(pc.Options["addr"], pc.Options["token"]), nil
	case "slack":
		return platforms.NewSlack(pc.Options["webhook_url"]), nil
	case "discord":
		return platforms.NewDiscord(pc.Options["webhook_url"]), nil
	case "mattermost":
		return platforms.NewMattermost(pc.Options["webhook_url"]), nil
	case "feishu":
		return platforms.NewFeishu(pc.Options["webhook_url"]), nil
	case "dingtalk":
		return platforms.NewDingTalk(pc.Options["webhook_url"]), nil
	case "wecom":
		return platforms.NewWeCom(pc.Options["webhook_url"]), nil
	case "email":
		return platforms.NewEmail(
			pc.Options["host"], pc.Options["port"],
			pc.Options["username"], pc.Options["password"],
			pc.Options["from"], pc.Options["to"],
		), nil
	case "sms":
		return platforms.NewSMS(
			pc.Options["account_sid"], pc.Options["auth_token"],
			pc.Options["from"], pc.Options["to"],
		), nil
	case "signal":
		return platforms.NewSignal(pc.Options["base_url"], pc.Options["account"]), nil
	case "whatsapp":
		return platforms.NewWhatsApp(pc.Options["phone_id"], pc.Options["access_token"]), nil
	case "matrix":
		return platforms.NewMatrix(
			pc.Options["home_server"], pc.Options["access_token"], pc.Options["room_id"],
		), nil
	case "homeassistant":
		return platforms.NewHomeAssistant(
			pc.Options["base_url"], pc.Options["access_token"], pc.Options["service"],
		), nil
	case "slack_events":
		return platforms.NewSlackEvents(pc.Options["addr"], pc.Options["bot_token"]), nil
	case "discord_bot":
		return platforms.NewDiscordBot(pc.Options["token"], pc.Options["channel_id"]), nil
	case "mattermost_bot":
		return platforms.NewMattermostBot(
			pc.Options["base_url"], pc.Options["token"], pc.Options["channel_id"],
		), nil
	default:
		return nil, fmt.Errorf("unknown platform type %q", t)
	}
}
