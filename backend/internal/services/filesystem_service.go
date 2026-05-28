package services

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemService manages local file operations under a storage directory.
type FileSystemService struct {
	storageDir string
}

// NewFileSystemService creates a new FileSystemService.
func NewFileSystemService(storageDir string) *FileSystemService {
	return &FileSystemService{storageDir: storageDir}
}

// LocalFile represents a file or folder in storage.
type LocalFile struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"` // "file" or "folder"
	Items []LocalFile `json:"items,omitempty"`
	Meta  *FileMeta   `json:"meta,omitempty"`
}

// FileMeta holds optional metadata for a file.
type FileMeta struct {
	PageContent string `json:"pageContent,omitempty"`
}

// ListLocalFiles lists all files and folders in the documents directory.
func (s *FileSystemService) ListLocalFiles(folderName string) ([]LocalFile, error) {
	docDir := filepath.Join(s.storageDir, "documents")
	if folderName != "" {
		docDir = filepath.Join(docDir, folderName)
	}

	entries, err := os.ReadDir(docDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []LocalFile{}, nil
		}
		return nil, err
	}

	var files []LocalFile
	for _, entry := range entries {
		lf := LocalFile{
			Name: entry.Name(),
			Type: "file",
		}
		if entry.IsDir() {
			lf.Type = "folder"
		}
		files = append(files, lf)
	}
	return files, nil
}

// CreateFolder creates a new folder under documents.
func (s *FileSystemService) CreateFolder(folderName string) error {
	p := filepath.Join(s.storageDir, "documents", folderName)
	return os.MkdirAll(p, 0755)
}

// RemoveFolder removes a folder and all its contents.
func (s *FileSystemService) RemoveFolder(folderName string) error {
	p := filepath.Join(s.storageDir, "documents", folderName)
	return os.RemoveAll(p)
}

// RemoveDocument removes a single document file.
func (s *FileSystemService) RemoveDocument(docName string) error {
	p := filepath.Join(s.storageDir, "documents", docName)
	return os.RemoveAll(p)
}

// AcceptedDocumentTypes returns a map of extension to MIME type for supported documents.
func (s *FileSystemService) AcceptedDocumentTypes() map[string]string {
	return map[string]string{
		".txt":  "text/plain",
		".md":   "text/markdown",
		".pdf":  "application/pdf",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".csv":  "text/csv",
		".json": "application/json",
		".html": "text/html",
		".htm":  "text/html",
	}
}

// GetDocumentPath returns the full path to a document.
func (s *FileSystemService) GetDocumentPath(docName string) string {
	return filepath.Join(s.storageDir, "documents", docName)
}

// SaveFile saves uploaded file content to the documents directory.
func (s *FileSystemService) SaveFile(folderName, filename string, reader io.Reader) (string, error) {
	dir := filepath.Join(s.storageDir, "documents")
	if folderName != "" {
		dir = filepath.Join(dir, folderName)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return "", err
	}
	return p, nil
}

// DetectMIME returns MIME type for a file path.
func (s *FileSystemService) DetectMIME(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

// AssetsDir returns the assets directory path.
func (s *FileSystemService) AssetsDir() string {
	return filepath.Join(s.storageDir, "assets")
}

// PfpDir returns the profile pictures directory path.
func (s *FileSystemService) PfpDir() string {
	return filepath.Join(s.storageDir, "assets", "pfp")
}

// SaveAsset saves uploaded file to assets directory.
func (s *FileSystemService) SaveAsset(filename string, reader io.Reader) (string, error) {
	dir := s.AssetsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return "", err
	}
	return p, nil
}

// SavePfp saves uploaded profile picture to pfp directory.
func (s *FileSystemService) SavePfp(filename string, reader io.Reader) (string, error) {
	dir := s.PfpDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		return "", err
	}
	return p, nil
}

// ReadAsset reads an asset file and returns its contents, size, and MIME type.
func (s *FileSystemService) ReadAsset(assetPath string) (found bool, data []byte, size int64, mimeType string, err error) {
	info, err := os.Stat(assetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, 0, "none/none", nil
		}
		return false, nil, 0, "", err
	}
	data, err = os.ReadFile(assetPath)
	if err != nil {
		return false, nil, 0, "", err
	}
	mimeType = s.DetectMIME(assetPath)
	return true, data, info.Size(), mimeType, nil
}

// RemoveAsset removes an asset file.
func (s *FileSystemService) RemoveAsset(assetPath string) error {
	return os.Remove(assetPath)
}

// IsWithin checks if target path is within base path (prevents directory traversal).
func (s *FileSystemService) IsWithin(basePath, targetPath string) bool {
	baseAbs, err1 := filepath.Abs(basePath)
	targetAbs, err2 := filepath.Abs(targetPath)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.HasPrefix(targetAbs, baseAbs)
}

// RenameAsset renames an asset file within the assets directory.
func (s *FileSystemService) RenameAsset(oldName, newName string) (string, error) {
	assetsDir := s.AssetsDir()
	oldPath := filepath.Join(assetsDir, oldName)
	newPath := filepath.Join(assetsDir, newName)
	if !s.IsWithin(assetsDir, oldPath) || !s.IsWithin(assetsDir, newPath) {
		return "", fmt.Errorf("invalid file path")
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", err
	}
	return newName, nil
}
