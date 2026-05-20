package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/odysseythink/hermind/tool/document"
)

func (s *Server) handleGeneratedFileDownload(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	// Reject path traversal attempts explicitly
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	mgr := document.NewManager(s.generatedFilesDir())
	file, err := mgr.Get(filename)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	if file == nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(filename)
	if len(ext) > 0 {
		ext = ext[1:]
	}
	mimeType := mgr.MimeType(ext)

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Buffer)))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Write(file.Buffer)
}

func (s *Server) generatedFilesDir() string {
	return filepath.Join(s.opts.InstanceRoot, "generated-files")
}
