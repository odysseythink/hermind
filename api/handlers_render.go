package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
)

type renderRequest struct {
	Content string `json:"content"`
}

type renderResponse struct {
	HTML string `json:"html"`
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req renderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
			),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(req.Content), &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := renderResponse{HTML: buf.String()}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
