// Package api — handler for the Anthropic-compatible /v1/messages proxy endpoint.
// Pure transport-layer translator; does not touch state.db, memory,
// skills, or the agent loop.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/anthropic"
)

const v1MessagesMaxBodyBytes = 10 << 20 // 10 MB

// handleV1Messages serves POST /v1/messages.
func (s *Server) handleV1Messages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	prov := s.opts.Deps.Provider
	if prov == nil {
		writeAnthropicError(w, http.StatusServiceUnavailable, "service_unavailable", "no LLM provider configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, v1MessagesMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// MaxBytesReader returns a "request body too large" error.
		writeAnthropicError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body too large")
		return
	}

	req, requestModel, stream, err := anthropic.Inbound(body)
	if err != nil {
		code := anthropic.InvalidErrorCode(err)
		if code == "" {
			code = "invalid_request_error"
		}
		writeAnthropicError(w, http.StatusBadRequest, code, err.Error())
		return
	}

	if stream {
		s.serveV1MessagesStream(w, r, prov, req, requestModel, start)
		return
	}

	resp, err := prov.Complete(r.Context(), req)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}

	bytesOut, err := anthropic.Outbound(resp, requestModel, anthropic.NewMsgID())
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-hermind-actual-model", resp.Model)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytesOut)

	slog.Info("v1.messages.request",
		"request_model", requestModel,
		"actual_model", resp.Model,
		"stream", false,
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"duration_ms", time.Since(start).Milliseconds(),
		"status", http.StatusOK)
}

// serveV1MessagesStream is the streaming branch (Task 7).
func (s *Server) serveV1MessagesStream(
	w http.ResponseWriter, r *http.Request,
	p provider.Provider, req *provider.Request, requestModel string, start time.Time,
) {
	stream, err := p.Stream(r.Context(), req)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}

	keepAlive := time.Duration(s.opts.Config.Proxy.KeepAliveSeconds) * time.Second
	if keepAlive <= 0 {
		keepAlive = 15 * time.Second
	}

	msgID := anthropic.NewMsgID()
	w.Header().Set("x-hermind-actual-model", req.Model)
	if err := anthropic.StreamOutbound(r.Context(), w, stream, requestModel, msgID, keepAlive); err != nil {
		// At this point headers are already written; just log.
		slog.Warn("v1.messages.stream_error", "err", err)
	}
	slog.Info("v1.messages.request",
		"request_model", requestModel,
		"stream", true,
		"duration_ms", time.Since(start).Milliseconds(),
		"status", http.StatusOK)
}

// writeAnthropicError writes an Anthropic-format error envelope.
func writeAnthropicError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errType,
			"message": message,
		},
	})
}

// errProviderNoStreaming is a typed sentinel for "provider lacks streaming
// but client requested it". Returned by serveV1MessagesStream's gate when
// applicable. Reserved for Task 9 / 10.
type errProviderNoStreaming struct{}

func (errProviderNoStreaming) Error() string { return "provider does not support streaming" }

// writeProviderError handles upstream provider failures.
func (s *Server) writeProviderError(w http.ResponseWriter, err error) {
	slog.Warn("v1.messages.provider_error", "err", err)
	msg := err.Error()

	if errors.Is(err, errProviderNoStreaming{}) {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "provider does not support streaming")
		return
	}

	// Best-effort classification by substring. Provider implementations
	// don't carry typed status codes today; this is a defensive sniff.
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "429") || strings.Contains(lower, "rate limit"):
		writeAnthropicError(w, http.StatusTooManyRequests, "rate_limit_error", msg)
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		writeAnthropicError(w, http.StatusUnauthorized, "authentication_error", msg)
	default:
		writeAnthropicError(w, http.StatusBadGateway, "api_error", msg)
	}
}
