// Package webconfig serves a browser-based editor for ~/.hermind/config.yaml.
// It binds loopback-only and assumes a single-user machine: no auth.
package webconfig

import (
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
	mux.HandleFunc("/api/save", s.handleSave)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
	return mux
}

// Serve binds addr and serves until shutdown is requested.
func Serve(path, addr string) error {
	s, err := New(path)
	if err != nil {
		return err
	}
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	err = s.srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

