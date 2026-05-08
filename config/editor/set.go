package editor

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Set assigns a scalar value at dotPath, creating intermediate mappings as
// needed. If the existing node at dotPath is a scalar, its Value and Tag
// are updated in place so line/column comments stick.
func (d *Doc) Set(dotPath string, value any) error {
	cur := d.documentContent()
	if cur == nil {
		d.root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		cur = d.root.Content[0]
	}
	segs := strings.Split(dotPath, ".")
	for i, seg := range segs {
		last := i == len(segs)-1
		if cur.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: %s: segment %q traverses non-map", dotPath, seg)
		}
		idx := indexOfKey(cur, seg)
		if idx < 0 {
			// Append new key/value pair.
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			var v *yaml.Node
			if last {
				v = scalarFromAny(value)
			} else {
				v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			cur.Content = append(cur.Content, k, v)
			cur = v
			continue
		}
		v := cur.Content[idx+1]
		if last {
			if v.Kind == yaml.ScalarNode {
				sv := scalarFromAny(value)
				v.Tag = sv.Tag
				v.Value = sv.Value
				v.Style = sv.Style
			} else {
				cur.Content[idx+1] = scalarFromAny(value)
			}
			return nil
		}
		if v.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: %s: segment %q exists but is not a map", dotPath, seg)
		}
		cur = v
	}
	return nil
}

// Remove deletes the key addressed by dotPath. Silently succeeds if the
// path does not exist.
func (d *Doc) Remove(dotPath string) error {
	segs := strings.Split(dotPath, ".")
	parent := d.documentContent()
	if parent == nil {
		return nil
	}
	for _, seg := range segs[:len(segs)-1] {
		idx := indexOfKey(parent, seg)
		if idx < 0 {
			return nil
		}
		parent = parent.Content[idx+1]
		if parent.Kind != yaml.MappingNode {
			return nil
		}
	}
	last := segs[len(segs)-1]
	idx := indexOfKey(parent, last)
	if idx < 0 {
		return nil
	}
	parent.Content = append(parent.Content[:idx], parent.Content[idx+2:]...)
	return nil
}

// SetBlock parses a YAML mapping fragment and attaches it as the value at
// dotPath, replacing anything already there.
func (d *Doc) SetBlock(dotPath, fragment string) error {
	var tmp yaml.Node
	if err := yaml.Unmarshal([]byte(fragment), &tmp); err != nil {
		return fmt.Errorf("editor: SetBlock %s: parse fragment: %w", dotPath, err)
	}
	if tmp.Kind != yaml.DocumentNode || len(tmp.Content) == 0 {
		return fmt.Errorf("editor: SetBlock %s: empty fragment", dotPath)
	}
	newNode := tmp.Content[0]

	cur := d.documentContent()
	if cur == nil {
		d.root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		cur = d.root.Content[0]
	}
	segs := strings.Split(dotPath, ".")
	for i, seg := range segs {
		last := i == len(segs)-1
		if cur.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: SetBlock %s: non-map in path", dotPath)
		}
		idx := indexOfKey(cur, seg)
		if last {
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			if idx < 0 {
				cur.Content = append(cur.Content, k, newNode)
			} else {
				cur.Content[idx+1] = newNode
			}
			return nil
		}
		if idx < 0 {
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			cur.Content = append(cur.Content, k, v)
			cur = v
		} else {
			cur = cur.Content[idx+1]
		}
	}
	return nil
}

func indexOfKey(mapNode *yaml.Node, key string) int {
	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// MapKeys returns the keys of the mapping at dotPath, or nil if the path
// is missing or not a mapping.
func (d *Doc) MapKeys(dotPath string) []string {
	n := d.lookupNode(dotPath)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	out := make([]string, 0, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

func scalarFromAny(v any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode}
	switch x := v.(type) {
	case string:
		n.Tag = "!!str"
		n.Value = x
	case bool:
		n.Tag = "!!bool"
		if x {
			n.Value = "true"
		} else {
			n.Value = "false"
		}
	case int:
		n.Tag = "!!int"
		n.Value = fmt.Sprintf("%d", x)
	case int64:
		n.Tag = "!!int"
		n.Value = fmt.Sprintf("%d", x)
	case float64:
		n.Tag = "!!float"
		n.Value = fmt.Sprintf("%v", x)
	default:
		n.Tag = "!!str"
		n.Value = fmt.Sprintf("%v", x)
	}
	return n
}
