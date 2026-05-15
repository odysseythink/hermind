// Package tool re-exports the pantheon tool registry types so existing
// hermind tool subpackages continue to compile. New code should import
// github.com/odysseythink/pantheon/tool directly.
package tool

import ptool "github.com/odysseythink/pantheon/tool"

type (
	Handler   = ptool.Handler
	CheckFunc = ptool.CheckFunc
	Entry     = ptool.Entry
	Registry  = ptool.Registry
)

// NewRegistry constructs an empty registry.
func NewRegistry() *Registry { return ptool.NewRegistry() }
