// Package editor owns the YAML AST of hermind's config file. It exposes
// Get/Set/Remove operations that preserve comments, ordering, and blank
// lines, and a Schema() catalog that describes every editable field so
// both the TUI and Web config UIs can render forms from the same source.
package editor

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Doc is a mutable handle on a YAML config file. Zero value is not usable;
// obtain one via Load.
type Doc struct {
	root *yaml.Node
	path string
}

// Load parses the YAML file at path. If the file does not exist, returns
// a Doc with an empty mapping root so callers can populate it and Save.
func Load(path string) (*Doc, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var root yaml.Node
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &root); err != nil {
			return nil, err
		}
	} else {
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}}
	}
	return &Doc{root: &root, path: path}, nil
}

// Path returns the file path this Doc will Save to.
func (d *Doc) Path() string { return d.path }

// Save atomically writes the current AST back to disk.
func (d *Doc) Save() error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d.root); err != nil { return err }
	if err := enc.Close(); err != nil { return err }

	dir := filepath.Dir(d.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil { return err }
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close(); os.Remove(tmpName); return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close(); os.Remove(tmpName); return err
	}
	if err := tmp.Close(); err != nil { os.Remove(tmpName); return err }
	return os.Rename(tmpName, d.path)
}
