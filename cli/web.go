package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/odysseythink/hermind/api"
)

// webRunOptions parameterize runWeb. Shared by newWebCmd, newRunCmd,
// and the bare `hermind` RunE.
type webRunOptions struct {
	Addr      string
	NoBrowser bool
	ExitAfter time.Duration
	// Out is where the listening-URL banner lines are written. Nil
	// defaults to os.Stdout. Tests inject a buffer to capture the output.
	Out io.Writer
}

// runWeb is the actual body of `hermind web`. Shared by newRunCmd and
// the root command's default RunE.
func runWeb(ctx context.Context, app *App, opts webRunOptions) error {
	if err := ensureStorage(app); err != nil {
		return err
	}

	deps, cleanup, err := BuildEngineDeps(ctx, app)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil && !errors.Is(err, errMissingAPIKey) {
		return fmt.Errorf("web: build engine deps: %w", err)
	}

	streams := api.NewMemoryStreamHub()
	srv, err := api.NewServer(&api.ServerOpts{
		Config:       app.Config,
		ConfigPath:   app.ConfigPath,
		InstanceRoot: app.InstanceRoot,
		Storage:      app.Storage,
		Version:      Version,
		Streams:      streams,
		Deps:         deps,
	})
	if err != nil {
		return err
	}

	var ln net.Listener
	if opts.Addr == "" {
		ln, err = listenRandomLocalhost()
		if err != nil {
			return fmt.Errorf("web: %w", err)
		}
	} else {
		ln, err = net.Listen("tcp", opts.Addr)
		if err != nil {
			return fmt.Errorf("web: listen %s: %w", opts.Addr, err)
		}
	}
	realAddr := "http://" + ln.Addr().String()
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	fmt.Fprintf(out, "hermind web listening on %s\n", realAddr)
	fmt.Fprintf(out, "instance:  %s\n", app.InstanceRoot)
	fmt.Fprintf(out, "open:      %s/\n", realAddr)

	if !opts.NoBrowser {
		go openBrowser(realAddr + "/")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if opts.ExitAfter > 0 {
		time.AfterFunc(opts.ExitAfter, cancel)
	}

	httpSrv := &http.Server{
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-runCtx.Done()
		shutCtx, c2 := context.WithTimeout(context.Background(), 2*time.Second)
		defer c2()
		_ = httpSrv.Shutdown(shutCtx)
	}()
	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// newWebCmd builds the `hermind web` subcommand. It parses flags,
// then delegates to runWeb.
func newWebCmd(app *App) *cobra.Command {
	var opts webRunOptions
	c := &cobra.Command{
		Use:   "web",
		Short: "Start the hermind web UI and REST API",
		Long: `Start the hermind web UI and REST API.

Binds to 127.0.0.1 by default on a random port in [30000,40000). Use
--addr to pin a specific host:port (useful for bookmarks or reverse
proxies).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Out = cmd.OutOrStdout()
			return runWeb(cmd.Context(), app, opts)
		},
	}
	c.Flags().StringVar(&opts.Addr, "addr", "",
		"bind address; empty = random port in [30000,40000) on 127.0.0.1")
	c.Flags().BoolVar(&opts.NoBrowser, "no-browser", false,
		"do not open the browser automatically")
	c.Flags().DurationVar(&opts.ExitAfter, "exit-after", 0,
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
