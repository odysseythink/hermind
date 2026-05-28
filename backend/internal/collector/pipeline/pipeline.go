package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/odysseythink/hermind/backend/internal/collector/utils"
)

var reservedFiles = []string{"__HOTDIR__.md"}

// ProcessFileOptions holds per-request options for ProcessFile.
type ProcessFileOptions struct {
	ParseOnly    bool
	AbsolutePath string
	Options      core.Options
}

// ExtractorRegistry is the minimal interface Pipeline needs from a registry.
type ExtractorRegistry interface {
	Get(ext string) ContentExtractor
}

// Pipeline orchestrates document processing from extraction through persistence.
type Pipeline struct {
	storageDir    string
	watchDir      string
	registry      ExtractorRegistry
	textExtractor ContentExtractor
	writer        *Writer
	enricher      *Enricher
}

// NewPipeline creates a new Pipeline.
func NewPipeline(storageDir, watchDir string, enricher *Enricher) *Pipeline {
	return &Pipeline{
		storageDir: storageDir,
		watchDir:   watchDir,
		writer:     NewWriter(storageDir),
		enricher:   enricher,
	}
}

// SetRegistry sets the extractor registry used by ProcessFile.
func (p *Pipeline) SetRegistry(registry ExtractorRegistry) {
	p.registry = registry
}

// RegisterTextExtractor sets the fallback extractor used when a file has no
// registered extractor but is parseable as text.
func (p *Pipeline) RegisterTextExtractor(extractor ContentExtractor) {
	p.textExtractor = extractor
}

// ProcessFile validates, extracts, enriches, and persists a single file.
func (p *Pipeline) ProcessFile(ctx context.Context, targetFilename string, options ProcessFileOptions, metadata map[string]string) *core.ProcessResponse {
	var fullFilePath string
	if options.AbsolutePath != "" {
		fullFilePath = filepath.Clean(options.AbsolutePath)
	} else {
		fullFilePath = filepath.Join(p.watchDir, utils.NormalizePath(targetFilename))
	}
	fullFilePath = filepath.Clean(fullFilePath)

	// Path must be within watch directory unless an absolute path is provided.
	if options.AbsolutePath == "" && !utils.IsWithin(p.watchDir, fullFilePath) {
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    "Filename is a not a valid path to process.",
			Documents: nil,
		}
	}

	baseName := filepath.Base(targetFilename)
	for _, reserved := range reservedFiles {
		if baseName == reserved {
			return &core.ProcessResponse{
				Filename:  targetFilename,
				Success:   false,
				Reason:    "Filename is a reserved filename and cannot be processed.",
				Documents: nil,
			}
		}
	}

	if _, err := os.Stat(fullFilePath); os.IsNotExist(err) {
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    "File does not exist in upload directory.",
			Documents: nil,
		}
	}

	ext := strings.ToLower(filepath.Ext(fullFilePath))
	if ext == "" && strings.Contains(targetFilename, ".") {
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    "No file extension found. This file cannot be processed.",
			Documents: nil,
		}
	}

	var extractor ContentExtractor
	if p.registry != nil {
		extractor = p.registry.Get(ext)
	}
	if extractor == nil {
		if utils.IsTextType(fullFilePath) && p.textExtractor != nil {
			extractor = p.textExtractor
		} else {
			if options.AbsolutePath == "" {
				utils.TrashFile(fullFilePath)
			}
			return &core.ProcessResponse{
				Filename:  targetFilename,
				Success:   false,
				Reason:    fmt.Sprintf("File extension %s not supported for parsing and cannot be assumed as text file type.", ext),
				Documents: nil,
			}
		}
	}

	input := ExtractInput{
		FilePath: fullFilePath,
		Filename: targetFilename,
		Metadata: metadata,
		Options:  options.Options,
	}
	out, err := extractor.Extract(ctx, input)
	if err != nil {
		if options.AbsolutePath == "" {
			utils.TrashFile(fullFilePath)
		}
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    err.Error(),
			Documents: nil,
		}
	}

	if strings.TrimSpace(out.Content) == "" {
		if options.AbsolutePath == "" {
			utils.TrashFile(fullFilePath)
		}
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    fmt.Sprintf("No text content found in %s.", targetFilename),
			Documents: nil,
		}
	}

	doc := &core.Document{
		Name:        targetFilename,
		URL:         "file://" + fullFilePath,
		PageContent: out.Content,
	}
	p.enricher.Enrich(doc, out.Content, fullFilePath, metadata)

	slugName := utils.SlugifyFilename(targetFilename)
	savedDoc, err := p.writer.Write(doc, fmt.Sprintf("%s-%s", slugName, uuid.New().String()), options.ParseOnly)
	if err != nil {
		if options.AbsolutePath == "" {
			utils.TrashFile(fullFilePath)
		}
		return &core.ProcessResponse{
			Filename:  targetFilename,
			Success:   false,
			Reason:    err.Error(),
			Documents: nil,
		}
	}

	if options.AbsolutePath == "" {
		utils.TrashFile(fullFilePath)
	}

	return &core.ProcessResponse{
		Filename:  targetFilename,
		Success:   true,
		Reason:    "",
		Documents: []core.Document{*savedDoc},
	}
}
