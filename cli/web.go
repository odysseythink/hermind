package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/cli/gatewayctl"
	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/gateway"
)

// newWebCmd builds the `hermind web` subcommand. It starts the REST API
// server on 127.0.0.1 by default (web UI is not intended to be exposed
// to the network), prints the session token + URL, and optionally opens
// the browser.
func newWebCmd(app *App) *cobra.Command {
	var (
		addr      string
		noBrowser bool
		exitAfter time.Duration
	)
	c := &cobra.Command{
		Use:   "web",
		Short: "Start the hermind web UI and REST API",
		Long: `Start the hermind web UI and REST API.

Binds to 127.0.0.1 by default. A fresh session token is generated on
every boot and never persisted to disk; it is injected into the served
landing page so the browser can authenticate automatically.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensureStorage(app); err != nil {
				return err
			}

			ctrl := gatewayctl.New(app.Config, func(cfg config.Config) (*gateway.Gateway, error) {
				return BuildGateway(BuildGatewayDeps{Config: cfg})
			})
			if err := ctrl.Start(cmd.Context()); err != nil {
				return fmt.Errorf("web: start gateway controller: %w", err)
			}
			defer func() {
				shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
				defer c2()
				ctrl.Shutdown(shutCtx)
			}()

			token, err := api.GenerateToken()
			if err != nil {
				return fmt.Errorf("web: generate token: %w", err)
			}
			// Install an in-process stream hub so the WebSocket and
			// SSE endpoints have a fan-out to publish into. The
			// gateway / future chat runner calls api.BridgeEngineToHub
			// to forward agent.Engine events when it constructs an
			// Engine per session.
			streams := api.NewMemoryStreamHub()
			srv, err := api.NewServer(&api.ServerOpts{
				Config:     app.Config,
				ConfigPath: app.ConfigPath,
				Storage:    app.Storage,
				Token:      token,
				Version:    Version,
				Streams:    streams,
				Controller: ctrl,
			})
			if err != nil {
				return err
			}

			// Bind early so we can report the real port when :0 was
			// requested.
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("web: listen %s: %w", addr, err)
			}
			realAddr := "http://" + ln.Addr().String()
			fmt.Fprintf(cmd.OutOrStdout(), "hermind web listening on %s\n", realAddr)
			fmt.Fprintf(cmd.OutOrStdout(), "token: %s\n", token)
			fmt.Fprintf(cmd.OutOrStdout(), "open:  %s/?t=%s\n", realAddr, token)

			if !noBrowser {
				go openBrowser(realAddr + "/?t=" + token)
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			if exitAfter > 0 {
				time.AfterFunc(exitAfter, cancel)
			}

			httpSrv := &http.Server{
				Handler:           srv.Router(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				<-ctx.Done()
				shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
				defer c2()
				_ = httpSrv.Shutdown(shutCtx)
			}()
			if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:9119",
		"bind address (keep 127.0.0.1 unless you know what you're doing)")
	c.Flags().BoolVar(&noBrowser, "no-browser", false,
		"do not open the browser automatically")
	c.Flags().DurationVar(&exitAfter, "exit-after", 0,
		"exit after the given duration (0 = run until Ctrl-C)")
	return c
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return
	}
	_ = exec.Command(cmd, args...).Start()
}
