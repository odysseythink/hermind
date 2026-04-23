package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstanceRoot returns the absolute path to this hermind instance's
// root directory. Resolution order:
//
//  1. $HERMIND_HOME (honored verbatim — caller decides whether it
//     already ends in ".hermind" or not).
//  2. <cwd>/.hermind
//
// There is no home-directory fallback. Each working directory is its
// own hermind instance.
func InstanceRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("HERMIND_HOME")); v != "" {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("config: resolve cwd: %w", err)
	}
	return filepath.Join(cwd, ".hermind"), nil
}

// InstancePath joins one or more path components onto the instance root.
func InstancePath(parts ...string) (string, error) {
	root, err := InstanceRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, parts...)...), nil
}
