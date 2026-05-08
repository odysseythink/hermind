package skills

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// SlashHandler runs a slash command. It returns a reply text that
// the REPL should display (may be empty) and an error.
type SlashHandler func(ctx context.Context, args []string) (string, error)

// SlashCommand describes a single slash command.
type SlashCommand struct {
	Name        string
	Description string
	Handler     SlashHandler
	Source      string // "builtin", skill name, etc.
}

// SlashRegistry holds the set of registered slash commands.
type SlashRegistry struct {
	mu  sync.RWMutex
	cmd map[string]*SlashCommand
}

// NewSlashRegistry builds an empty slash registry.
func NewSlashRegistry() *SlashRegistry {
	return &SlashRegistry{cmd: make(map[string]*SlashCommand)}
}

// Register adds or replaces a command by name. Names should NOT
// include the leading slash.
func (r *SlashRegistry) Register(c *SlashCommand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cmd[c.Name] = c
}

// All returns every registered command sorted by name.
func (r *SlashRegistry) All() []*SlashCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*SlashCommand, 0, len(r.cmd))
	for _, c := range r.cmd {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Dispatch parses a REPL line (must start with "/"), looks up the
// command, and runs it. Returns (reply, true, err) when the input was
// a slash command, or (_, false, nil) when it was an ordinary message
// and should be passed to the agent instead.
func (r *SlashRegistry) Dispatch(ctx context.Context, line string) (string, bool, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return "", false, nil
	}
	parts := strings.Fields(line[1:])
	if len(parts) == 0 {
		return "", true, fmt.Errorf("empty slash command")
	}
	name := parts[0]
	args := parts[1:]

	r.mu.RLock()
	cmd, ok := r.cmd[name]
	r.mu.RUnlock()
	if !ok {
		return "", true, fmt.Errorf("unknown slash command: /%s", name)
	}
	reply, err := cmd.Handler(ctx, args)
	return reply, true, err
}
