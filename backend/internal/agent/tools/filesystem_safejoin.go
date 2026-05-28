package tools

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrPathEscape is returned when a user-supplied path escapes the sandbox root.
var ErrPathEscape = errors.New("path escapes filesystem root")

// safeJoin resolves userPath against root, defending against:
//   - parent-directory traversal (../)
//   - absolute paths in user input
//   - symlinks escaping the root after resolution
//
// Returns the absolute, symlink-resolved path that is guaranteed to be within root.
func safeJoin(root, userPath string) (string, error) {
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("%w: absolute paths not allowed", ErrPathEscape)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Resolve any symlinks in root once
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		// Root doesn't exist yet — use absRoot as-is (filesystem ops will fail later with a clearer error)
		resolvedRoot = absRoot
	}

	joined := filepath.Join(resolvedRoot, userPath)
	cleaned := filepath.Clean(joined)
	if !strings.HasPrefix(cleaned, resolvedRoot+string(filepath.Separator)) && cleaned != resolvedRoot {
		return "", fmt.Errorf("%w: %s outside %s", ErrPathEscape, cleaned, resolvedRoot)
	}
	// Resolve symlinks on the final path if it exists
	if final, err := filepath.EvalSymlinks(cleaned); err == nil {
		if !strings.HasPrefix(final, resolvedRoot+string(filepath.Separator)) && final != resolvedRoot {
			return "", fmt.Errorf("%w: symlink target %s outside %s", ErrPathEscape, final, resolvedRoot)
		}
		return final, nil
	}
	return cleaned, nil
}
