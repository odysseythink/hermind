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

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
	"github.com/odysseythink/hermind/gateway/platforms"
	"github.com/odysseythink/hermind/metrics"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/tracing"
	"github.com/odysseythink/hermind/provider/factory"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/file"
	"github.com/odysseythink/hermind/tool/terminal"
	"github.com/spf13/cobra"
)

// newGatewayCmd creates the "hermind gateway" subcommand.
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

	g, err := BuildGateway(BuildGatewayDeps{
		Config:  *app.Config,
		Primary: primary,
		Aux:     aux,
		Storage: app.Storage,
		Tools:   reg,
	})
	if err != nil {
		return err
	}

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
// The type is taken from pc.Type, or falls back to the map key when pc.Type
// is empty. Lookup is delegated to the gateway/platforms registry; each
// adapter self-registers via a descriptor_<type>.go init().
func buildPlatform(name string, pc config.PlatformConfig) (gateway.Platform, error) {
	t := strings.ToLower(pc.Type)
	if t == "" {
		t = strings.ToLower(name)
	}
	d, ok := platforms.Get(t)
	if !ok {
		return nil, fmt.Errorf("unknown platform type %q", t)
	}
	return d.Build(pc.Options)
}
