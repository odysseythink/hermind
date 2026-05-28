package mcp

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// shellEnv runs "$SHELL -ic env" to capture the user's interactive shell
// environment (PATH, NODE_PATH, language paths, etc) so subprocess MCP
// servers see the same toolchain a human shell would. On error or unsupported
// platforms, falls back to os.Environ().
func shellEnv(parent context.Context) map[string]string {
	if runtime.GOOS == "windows" {
		return osEnvMap()
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		return osEnvMap()
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-ic", "env")
	out, err := cmd.Output()
	if err != nil {
		return osEnvMap()
	}
	return parseEnvOutput(string(out))
}

func osEnvMap() map[string]string {
	m := make(map[string]string, len(os.Environ()))
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func parseEnvOutput(s string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		m[line[:i]] = line[i+1:]
	}
	return m
}

// buildServerEnv produces the KEY=VAL slice for exec.Cmd.Env, layering:
// 1. base shell environment (or os.Environ on fallback)
// 2. docker hardcoded defaults (if HERMIND_RUNTIME=docker)
// 3. user-specified server.Env (highest priority)
func buildServerEnv(srv *ServerConfig) []string {
	base := shellEnv(context.Background())
	if base["PATH"] == "" {
		base["PATH"] = "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
	}
	if base["NODE_PATH"] == "" {
		base["NODE_PATH"] = "/usr/local/lib/node_modules"
	}
	if os.Getenv("HERMIND_RUNTIME") == "docker" {
		if base["NODE_PATH"] == "" {
			base["NODE_PATH"] = "/usr/local/lib/node_modules"
		}
		if base["PATH"] == "" {
			base["PATH"] = "/usr/local/bin:/usr/bin:/bin"
		}
	}
	for k, v := range srv.Env {
		base[k] = v
	}
	out := make([]string, 0, len(base))
	for k, v := range base {
		out = append(out, k+"="+v)
	}
	return out
}
