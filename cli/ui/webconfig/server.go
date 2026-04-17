// Package webconfig serves a browser-based editor for ~/.hermind/config.yaml.
// It binds loopback-only and assumes a single-user machine: no auth.
package webconfig

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/config/editor"
)

//go:embed web/*
var webFS embed.FS

// Server wires editor.Doc to HTTP handlers + embedded static assets.
type Server struct {
	doc *editor.Doc
	srv *http.Server
}

// New loads path and prepares a Server.
func New(path string) (*Server, error) {
	doc, err := editor.Load(path)
	if err != nil {
		return nil, err
	}
	return &Server{doc: doc}, nil
}

// Handler returns the http.Handler for mounting.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/schema", s.handleSchema)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/providers", s.handleProviders)
	mux.HandleFunc("/api/providers/models", s.handleProvidersModels)
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
	return mux
}

// Serve binds addr and serves until ctx is cancelled or the in-browser
// "Save & Exit" action calls /api/shutdown.
func Serve(ctx context.Context, path, addr string) error {
	s, err := New(path)
	if err != nil {
		return err
	}
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		err := s.srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutdownCtx)
		<-errCh
		return nil
	case err := <-errCh:
		return err
	}
}

