package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

type renderMermaidRequest struct {
	Source string `json:"source"`
}

type renderMermaidResponse struct {
	SVG string `json:"svg"`
}

func handleRenderMermaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req renderMermaidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := exec.Command("mmdc", "-o", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fallbackMermaid(w)
		return
	}
	go func() {
		stdin.Write([]byte(req.Source))
		stdin.Close()
	}()
	out, err := cmd.Output()
	if err != nil {
		fallbackMermaid(w)
		return
	}

	resp := renderMermaidResponse{SVG: string(out)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func fallbackMermaid(w http.ResponseWriter) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="60"><text x="10" y="30" fill="#8a8680" font-family="monospace">[Mermaid diagram unavailable]</text></svg>`
	resp := renderMermaidResponse{SVG: svg}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
