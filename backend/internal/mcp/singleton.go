package mcp

import (
	"sync"

	"github.com/odysseythink/hermind/backend/internal/config"
)

var (
	instance *Hypervisor
	once     sync.Once
)

// Instance returns the process-wide MCP hypervisor singleton. First call
// initialises it from the provided config; subsequent calls return the same
// instance regardless of argument.
func Instance(cfg *config.Config) *Hypervisor {
	once.Do(func() { instance = newHypervisor(cfg) })
	return instance
}
