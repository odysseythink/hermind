package stdio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/provider"
)

// AgentVersion is reported back through the initialize response.
// It is a package-level var so `hermind acp` can set it from the
// build-stamped cli.Version without introducing an import cycle.
var AgentVersion = "dev"

// Handlers bundles the collaborators each ACP method needs.
type Handlers struct {
	Sessions *SessionManager

	// Factory produces a provider.Provider given a model ref
	// (e.g. "anthropic/claude-opus-4-6"). The caller typically passes
	// a closure that routes through provider/factory with the right
	// config.ProviderConfig.
	Factory func(model string) (provider.Provider, error)

	// AgentCfg reserves room for plumbing engine settings (max turns,
	// compression, etc.) into the prompt loop. Unused by the MVP.
	AgentCfg config.AgentConfig
}

// ---- initialize ----

func (h *Handlers) handleInitialize(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	r := initializeResult{
		ProtocolVersion: 1,
		AgentInfo: agentInfo{
			Name:    "hermind",
			Version: AgentVersion,
		},
		AgentCapability: agentCap{LoadSession: true},
	}
	return json.Marshal(r)
}

// ---- authenticate ----

// MVP does not advertise auth methods and so receives no authenticate
// call in practice. The stub returns the empty object for protocol
// compatibility with clients that send it eagerly.
func (h *Handlers) handleAuthenticate(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// ---- session/new + session/load ----

func (h *Handlers) handleNewSession(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p newSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Cwd == "" {
		return nil, errors.New("acp/stdio: new_session: cwd is required")
	}
	model := p.Model
	if model == "" {
		model = "anthropic/claude-opus-4-6"
	}
	s, err := h.Sessions.Create(ctx, p.Cwd, model)
	if err != nil {
		return nil, err
	}
	return json.Marshal(newSessionResult{SessionID: s.ID})
}

func (h *Handlers) handleLoadSession(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p loadSessionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	s, err := h.Sessions.Get(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if p.Cwd != "" {
		s.Cwd = p.Cwd
	}
	return json.RawMessage(`{}`), nil
}

// ---- prompt ----

func (h *Handlers) handlePrompt(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p promptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, errors.New("acp/stdio: prompt: sessionId is required")
	}

	s, err := h.Sessions.Get(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}

	text := extractText(p.Prompt)
	if text == "" {
		return json.Marshal(promptResult{StopReason: "refusal"})
	}
	if err := h.Sessions.AppendUserText(ctx, s.ID, text); err != nil {
		return nil, err
	}

	if h.Factory == nil {
		return nil, errors.New("acp/stdio: no provider factory configured")
	}
	prov, err := h.Factory(s.Model)
	if err != nil {
		return nil, fmt.Errorf("acp/stdio: provider factory: %w", err)
	}

	history, err := h.Sessions.History(ctx, s.ID)
	if err != nil {
		return nil, err
	}

	cctx, cancel := context.WithCancel(ctx)
	h.Sessions.SetCancel(s.ID, cancel)
	defer cancel()

	resp, err := prov.Complete(cctx, &provider.Request{
		Model:     s.Model,
		Messages:  history,
		MaxTokens: 4096,
	})
	if err != nil {
		if errors.Is(cctx.Err(), context.Canceled) {
			return json.Marshal(promptResult{StopReason: "cancelled"})
		}
		return nil, err
	}

	if err := h.Sessions.AppendAssistantText(ctx, s.ID, resp.Message.Content.Text()); err != nil {
		return nil, err
	}

	stop := resp.FinishReason
	if stop == "" {
		stop = "end_turn"
	}
	return json.Marshal(promptResult{StopReason: stop})
}

// ---- cancel ----

func (h *Handlers) handleCancel(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p cancelParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	h.Sessions.Cancel(p.SessionID)
	return json.RawMessage(`null`), nil
}

// extractText concatenates every text block into a single string.
// Non-text blocks (image, resource_link, audio) are ignored in the MVP.
func extractText(blocks []promptContentBlock) string {
	var total string
	for _, b := range blocks {
		if b.Type == "text" {
			total += b.Text
		}
	}
	return total
}
