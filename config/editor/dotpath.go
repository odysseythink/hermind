package editor

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// documentContent returns the first child of the DocumentNode wrapper,
// which is the actual mapping root. Callers should nil-check.
func (d *Doc) documentContent() *yaml.Node {
	if d.root == nil || d.root.Kind != yaml.DocumentNode || len(d.root.Content) == 0 {
		return nil
	}
	return d.root.Content[0]
}

// Get returns the scalar value at dotPath. The second return is false if
// the path does not resolve to a scalar.
func (d *Doc) Get(dotPath string) (string, bool) {
	n := d.lookupNode(dotPath)
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", false
	}
	return n.Value, true
}

// lookupNode walks dotPath against the document mapping. Returns nil if
// any segment is missing or a non-map is traversed.
func (d *Doc) lookupNode(dotPath string) *yaml.Node {
	cur := d.documentContent()
	if cur == nil {
		return nil
	}
	for _, seg := range strings.Split(dotPath, ".") {
		if cur.Kind != yaml.MappingNode {
			return nil
		}
		found := false
		for i := 0; i < len(cur.Content); i += 2 {
			k, v := cur.Content[i], cur.Content[i+1]
			if k.Value == seg {
				cur = v
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return cur
}
