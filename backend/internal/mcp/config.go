package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Path string
	mu   sync.Mutex
}

func NewConfig(storageDir string) *Config {
	return &Config{Path: filepath.Join(storageDir, "plugins", "anythingllm_mcp_servers.json")}
}

type rawFile struct {
	MCPServers map[string]*ServerConfig `json:"mcpServers"`
}

func (c *Config) Ensure() error {
	if _, err := os.Stat(c.Path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
		return err
	}
	return os.WriteFile(c.Path, []byte(`{"mcpServers":{}}`), 0644)
}

func (c *Config) Load() ([]ServerConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadLocked()
}

func (c *Config) loadLocked() ([]ServerConfig, error) {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []ServerConfig{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return []ServerConfig{}, nil
	}
	var raw rawFile
	if err := json.Unmarshal(data, &raw); err != nil {
		// Node parity: tolerate malformed JSON, treat as empty
		return []ServerConfig{}, nil
	}
	out := make([]ServerConfig, 0, len(raw.MCPServers))
	for name, srv := range raw.MCPServers {
		if srv == nil {
			continue
		}
		srv.Name = name
		out = append(out, *srv)
	}
	return out, nil
}

func (c *Config) writeLocked(servers []ServerConfig) error {
	raw := rawFile{MCPServers: make(map[string]*ServerConfig, len(servers))}
	for i := range servers {
		name := servers[i].Name
		srv := servers[i]
		srv.Name = "" // never serialise
		raw.MCPServers[name] = &srv
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		// Windows: rename over existing fails — fall back to direct write
		_ = os.Remove(tmp)
		return os.WriteFile(c.Path, data, 0644)
	}
	return nil
}

func (c *Config) Write(servers []ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeLocked(servers)
}

func (c *Config) RemoveServer(name string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	servers, err := c.loadLocked()
	if err != nil {
		return false, err
	}
	idx := -1
	for i, s := range servers {
		if s.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, nil
	}
	servers = append(servers[:idx], servers[idx+1:]...)
	if err := c.writeLocked(servers); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Config) UpdateSuppressedTools(serverName, toolName string, enabled bool) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	servers, err := c.loadLocked()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		if servers[i].Name != serverName {
			continue
		}
		if servers[i].AnythingLLM == nil {
			servers[i].AnythingLLM = &AnythingLLMOptions{}
		}
		suppressed := servers[i].AnythingLLM.SuppressedTools
		if enabled {
			suppressed = removeString(suppressed, toolName)
		} else if !containsString(suppressed, toolName) {
			suppressed = append(suppressed, toolName)
		}
		servers[i].AnythingLLM.SuppressedTools = suppressed
		if err := c.writeLocked(servers); err != nil {
			return nil, err
		}
		return suppressed, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
}

func (c *Config) GetSuppressedTools(serverName string) []string {
	servers, err := c.Load()
	if err != nil {
		return []string{}
	}
	for _, s := range servers {
		if s.Name != serverName {
			continue
		}
		if s.AnythingLLM == nil {
			return []string{}
		}
		if s.AnythingLLM.SuppressedTools == nil {
			return []string{}
		}
		return s.AnythingLLM.SuppressedTools
	}
	return []string{}
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func removeString(xs []string, x string) []string {
	out := xs[:0]
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}
