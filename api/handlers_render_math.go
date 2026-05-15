package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
)

type renderMathRequest struct {
	Source      string `json:"source"`
	DisplayMode bool   `json:"display_mode"`
}

type renderMathResponse struct {
	SVG string `json:"svg"`
}

func handleRenderMath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req renderMathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := exec.Command("katex", "--format", "svg")
	if req.DisplayMode {
		cmd.Args = append(cmd.Args, "--display-mode")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fallbackMath(w, req.Source)
		return
	}
	go func() {
		stdin.Write([]byte(req.Source))
		stdin.Close()
	}()
	out, err := cmd.Output()
	if err != nil {
		fallbackMath(w, req.Source)
		return
	}

	resp := renderMathResponse{SVG: string(out)}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func fallbackMath(w http.ResponseWriter, source string) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="400" height="40"><text x="10" y="25" fill="#569cd6" font-family="monospace">` + source + `</text></svg>`
	resp := renderMathResponse{SVG: svg}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
