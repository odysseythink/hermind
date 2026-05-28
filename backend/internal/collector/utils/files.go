package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// Default file/directory permissions.
const (
	DirPerm  = 0o755
	FilePerm = 0o644
)

// WriteToServerDocuments writes a document as JSON to the server documents folder.
// If parseOnly is true it writes to storageDir/direct-uploads, otherwise to
// storageDir/documents/custom-documents. The filename gets a .json suffix.
func WriteToServerDocuments(storageDir string, doc *core.Document, filename string, parseOnly bool) (*core.Document, error) {
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	var destination string
	if parseOnly {
		destination = filepath.Join(storageDir, "direct-uploads")
	} else {
		destination = filepath.Join(storageDir, "documents", "custom-documents")
	}

	if err := os.MkdirAll(destination, DirPerm); err != nil {
		return nil, fmt.Errorf("create destination directory: %w", err)
	}

	safeFilename := SanitizeFileName(filename)
	if !strings.HasSuffix(safeFilename, ".json") {
		safeFilename += ".json"
	}

	destinationFilePath := filepath.Join(destination, safeFilename)
	data, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("marshal document: %w", err)
	}

	if err := os.WriteFile(destinationFilePath, data, FilePerm); err != nil {
		return nil, fmt.Errorf("write document file: %w", err)
	}

	// location is the last two path segments
	parts := strings.Split(filepath.ToSlash(destinationFilePath), "/")
	if len(parts) >= 2 {
		doc.Location = strings.Join(parts[len(parts)-2:], "/")
	} else {
		doc.Location = destinationFilePath
	}
	doc.IsDirectUpload = parseOnly
	return doc, nil
}

// TrashFile removes the file at the given path if it exists and is not a directory.
func TrashFile(filepath string) {
	info, err := os.Stat(filepath)
	if err != nil {
		return
	}
	if info.IsDir() {
		return
	}
	_ = os.Remove(filepath)
}

// CreatedDate returns the birth time of the file as a locale-formatted string.
// If unavailable it returns "unknown".
func CreatedDate(filepath string) string {
	info, err := os.Stat(filepath)
	if err != nil {
		return "unknown"
	}
	birthTime := info.ModTime()
	// On Unix, birth time may not be available; fall back to ModTime.
	// Use a simple locale string similar to JS toLocaleString().
	return birthTime.Format("2006-01-02 15:04:05")
}

var leadingParentRe = regexp.MustCompile(`^(\.\.(?:/|\\|$))+`)

// NormalizePath cleans a file path and strips leading parent-directory traversals.
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	result := filepath.Clean(p)
	result = leadingParentRe.ReplaceAllString(result, "")
	result = strings.TrimSpace(result)
	if result == "" || result == "." || result == "/" || result == string(filepath.Separator) {
		return ""
	}
	return result
}

// IsWithin returns true if inner is inside outer (not equal to outer).
func IsWithin(outer, inner string) bool {
	outer = filepath.Clean(outer)
	inner = filepath.Clean(inner)
	if outer == inner {
		return false
	}
	rel, err := filepath.Rel(outer, inner)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// SanitizeFileName strips characters that are illegal in filenames.
func SanitizeFileName(fileName string) string {
	if fileName == "" {
		return fileName
	}
	// Strip illegal Windows filename characters + Unicode quotation marks.
	illegal := []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*',
		'\u201C', '\u201D', '\u201E', '\u201F', '\u2018', '\u2019', '\u201A', '\u201B'}
	for _, c := range illegal {
		fileName = strings.ReplaceAll(fileName, string(c), "")
	}
	return fileName
}

// SlugifyFilename returns a URL-friendly slug of the filename.
func SlugifyFilename(fileName string) string {
	return slug.Make(fileName)
}
