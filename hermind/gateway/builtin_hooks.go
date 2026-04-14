package gateway

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RateLimitHook drops messages from the same (platform, user) key
// that arrive faster than the given cooldown. Useful to stop
// runaway loops or abusive clients.
func RateLimitHook(cooldown time.Duration) PreHook {
	var mu sync.Mutex
	last := map[string]time.Time{}
	return func(ctx context.Context, in *IncomingMessage) (*IncomingMessage, error) {
		key := in.Platform + ":" + in.UserID
		mu.Lock()
		defer mu.Unlock()
		if prev, ok := last[key]; ok {
			if time.Since(prev) < cooldown {
				return nil, nil // silently drop
			}
		}
		last[key] = time.Now()
		return in, nil
	}
}

// UserBanHook drops messages from users in the ban list. Keys can
// be either "<platform>:<user>" or bare "<user>" (match across all
// platforms).
func UserBanHook(banned []string) PreHook {
	set := map[string]bool{}
	for _, b := range banned {
		set[strings.TrimSpace(b)] = true
	}
	return func(ctx context.Context, in *IncomingMessage) (*IncomingMessage, error) {
		if set[in.UserID] || set[in.Platform+":"+in.UserID] {
			return nil, fmt.Errorf("user banned: %s", in.UserID)
		}
		return in, nil
	}
}

// CommandAllowlistHook accepts only messages whose first token
// matches one of the allowlisted commands. An empty allowlist means
// everything is allowed.
func CommandAllowlistHook(commands []string) PreHook {
	set := map[string]bool{}
	for _, c := range commands {
		set[strings.TrimSpace(c)] = true
	}
	return func(ctx context.Context, in *IncomingMessage) (*IncomingMessage, error) {
		if len(set) == 0 {
			return in, nil
		}
		parts := strings.Fields(strings.TrimSpace(in.Text))
		if len(parts) == 0 || !set[parts[0]] {
			return nil, nil
		}
		return in, nil
	}
}

// ProfanityRedactHook replaces occurrences of banned substrings in
// the outgoing reply with ****. This is a deliberately blunt filter
// — real moderation should run upstream.
func ProfanityRedactHook(banned []string) PostHook {
	return func(ctx context.Context, in IncomingMessage, out *OutgoingMessage) (*OutgoingMessage, error) {
		text := out.Text
		for _, b := range banned {
			if b == "" {
				continue
			}
			text = strings.ReplaceAll(text, b, strings.Repeat("*", len(b)))
		}
		out.Text = text
		return out, nil
	}
}
