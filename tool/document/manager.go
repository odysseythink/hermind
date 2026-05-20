package document

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Manager handles saving and retrieving generated files.
type Manager struct {
	outputDir string
}

// SavedFile holds metadata for a saved generated file.
type SavedFile struct {
	Filename        string
	DisplayFilename string
	FileSize        int64
	StoragePath     string
}

// RetrievedFile holds the content of a retrieved file.
type RetrievedFile struct {
	Buffer      []byte
	StoragePath string
}

var filenameRegex = regexp.MustCompile(`^([a-z]+)-([a-f0-9-]{36})\.(\w+)$`)

// NewManager creates a Manager that stores files in outputDir.
func NewManager(outputDir string) *Manager {
	return &Manager{outputDir: outputDir}
}

// EnsureDir creates the output directory if it does not exist.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.outputDir, 0o755)
}

// Save writes a file to storage and returns metadata.
func (m *Manager) Save(fileType, extension string, buffer []byte, displayFilename string) (*SavedFile, error) {
	if err := m.EnsureDir(); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	filename := fmt.Sprintf("%s-%s.%s", fileType, uuid.NewString(), extension)
	storagePath := filepath.Join(m.outputDir, filename)
	if err := os.WriteFile(storagePath, buffer, 0o644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}
	return &SavedFile{
		Filename:        filename,
		DisplayFilename: displayFilename,
		FileSize:        int64(len(buffer)),
		StoragePath:     storagePath,
	}, nil
}

// Get retrieves a generated file by its storage filename.
func (m *Manager) Get(filename string) (*RetrievedFile, error) {
	if !filenameRegex.MatchString(filename) {
		return nil, nil
	}
	storagePath := filepath.Join(m.outputDir, filename)
	// Defensive: ensure resolved path is still inside outputDir
	if !isSubpath(storagePath, m.outputDir) {
		return nil, nil
	}
	buf, err := os.ReadFile(storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &RetrievedFile{Buffer: buf, StoragePath: storagePath}, nil
}

// MimeType returns the MIME type for a file extension.
func (m *Manager) MimeType(ext string) string {
	switch ext {
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pdf":
		return "application/pdf"
	case "txt", "md", "csv", "json", "html", "xml", "yaml", "log":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func isSubpath(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return !path.IsAbs(rel) && rel != ".." && !containsDotDot(rel)
}

func containsDotDot(p string) bool {
	parts := strings.Split(p, string(filepath.Separator))
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}
