package document

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/odysseythink/hermind/tool"
)

// NodeJSWrapper invokes the Node.js document generation scripts.
type NodeJSWrapper struct {
	scriptDir string
	outputDir string
}

// NewNodeJSWrapper creates a wrapper. scriptDir is the directory containing
// the document-scripts package (where bin/generate-doc.js lives).
func NewNodeJSWrapper(scriptDir, outputDir string) *NodeJSWrapper {
	return &NodeJSWrapper{
		scriptDir: scriptDir,
		outputDir: outputDir,
	}
}

// Generate invokes the Node.js script for the given type with the given params.
// Returns the absolute path to the generated file.
func (w *NodeJSWrapper) Generate(ctx context.Context, docType string, params map[string]interface{}) (string, error) {
	scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("node.js script not found at %s: %w", scriptPath, err)
	}

	params["type"] = docType
	params["outputDir"] = w.outputDir

	jsonArgs, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}

	// Apply a hard timeout to prevent indefinite hangs
	const maxTimeout = 30 * time.Second
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, maxTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "node", scriptPath)
	cmd.Dir = w.scriptDir
	cmd.Stdin = strings.NewReader(string(jsonArgs))
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("node.js error: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("node.js subprocess failed: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", fmt.Errorf("node.js script returned empty path")
	}

	// Validate the returned path is within the expected output directory
	if !isSubpath(path, w.outputDir) {
		return "", fmt.Errorf("node.js script returned path outside output directory: %s", path)
	}

	return path, nil
}

// IsAvailable returns true if Node.js and the script are available.
func (w *NodeJSWrapper) IsAvailable() bool {
	scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return false
	}
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	return true
}

// NewCreateWordHandler returns a handler for create_word_document.
func NewCreateWordHandler(wrapper *NodeJSWrapper) tool.Handler {
	mgr := NewManager(wrapper.outputDir)
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.ToolError(fmt.Sprintf("invalid args: %v", err)), nil
		}
		filename, _ := params["filename"].(string)
		if filename == "" {
			filename = "document.docx"
		}
		if !strings.HasSuffix(strings.ToLower(filename), ".docx") {
			filename += ".docx"
		}
		params["filename"] = filename

		path, err := wrapper.Generate(ctx, "docx", params)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("generate docx: %v", err)), nil
		}
		defer os.Remove(path)

		// Read the generated file
		buf, err := os.ReadFile(path)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("read generated file: %v", err)), nil
		}

		// Move to managed storage with proper filename
		saved, err := mgr.Save("docx", "docx", buf, filename)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created Word document '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

// NewCreatePPTXHandler returns a handler for create_pptx_presentation.
func NewCreatePPTXHandler(wrapper *NodeJSWrapper) tool.Handler {
	mgr := NewManager(wrapper.outputDir)
	return func(ctx context.Context, args json.RawMessage) (string, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.ToolError(fmt.Sprintf("invalid args: %v", err)), nil
		}
		filename, _ := params["filename"].(string)
		if filename == "" {
			filename = "presentation.pptx"
		}
		if !strings.HasSuffix(strings.ToLower(filename), ".pptx") {
			filename += ".pptx"
		}
		params["filename"] = filename

		path, err := wrapper.Generate(ctx, "pptx", params)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("generate pptx: %v", err)), nil
		}
		defer os.Remove(path)

		buf, err := os.ReadFile(path)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("read generated file: %v", err)), nil
		}

		saved, err := mgr.Save("pptx", "pptx", buf, filename)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created PowerPoint presentation '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}
