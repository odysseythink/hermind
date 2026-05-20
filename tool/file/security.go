package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// validatePath ensures the given path is safe and within allowed directories.
// It resolves symlinks, rejects traversal attempts, and checks the whitelist.
func validatePath(input string, allowed []string) error {
	if input == "" {
		return fmt.Errorf("path is required")
	}
	if len(allowed) == 0 {
		return fmt.Errorf("allowed directories not configured; please set allowed directories in Settings -> Tools -> Filesystem")
	}

	if strings.Contains(input, "..") {
		return fmt.Errorf("path contains traversal segment '..'")
	}

	abs, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			// The path itself does not exist. Check if it's a symlink
			// pointing outside allowed directories before falling back
			// to validating the parent directory.
			if info, lerr := os.Lstat(abs); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
				target, lerr := os.Readlink(abs)
				if lerr != nil {
					return fmt.Errorf("failed to read symlink: %w", lerr)
				}
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(abs), target)
				}
				resolvedTarget, lerr := filepath.EvalSymlinks(target)
				if lerr != nil {
					return fmt.Errorf("failed to resolve symlink target: %w", lerr)
				}
				resolved = resolvedTarget
			} else {
				parent := filepath.Dir(abs)
				resolvedParent, err := filepath.EvalSymlinks(parent)
				if err != nil {
					return fmt.Errorf("failed to resolve parent directory: %w", err)
				}
				resolved = resolvedParent
			}
		} else {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}

	for _, dir := range allowed {
		allowedAbs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		allowedAbs, err = filepath.EvalSymlinks(allowedAbs)
		if err != nil {
			continue
		}
		prefix := allowedAbs
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if resolved == allowedAbs || strings.HasPrefix(resolved+string(filepath.Separator), prefix) {
			return nil
		}
	}

	return fmt.Errorf("path %q is outside allowed directories", input)
}

// getAllowedDirs reads allowed directories from config settings.
func getAllowedDirs(cfg map[string]any) []string {
	raw, ok := cfg["allowed_directories"].(string)
	if !ok || raw == "" {
		return nil
	}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
