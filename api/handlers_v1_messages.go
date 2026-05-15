// Package api — handler for the Anthropic-compatible /v1/messages proxy endpoint.
// Pure transport-layer translator; does not touch state.db, memory,
// skills, or the agent loop.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/provider"
	"github.com/odysseythink/hermind/provider/anthropic"
	"github.com/odysseythink/mlog"
	"github.com/odysseythink/pantheon/core"
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

	pantheonReq := providerRequestToPantheon(req)
	resp, err := prov.Generate(r.Context(), pantheonReq)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}

	providerResp := pantheonResponseToProvider(resp)
	bytesOut, err := anthropic.Outbound(providerResp, requestModel, anthropic.NewMsgID())
	if err != nil {
		writeAnthropicError(w, http.StatusInternalServerError, "api_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("x-hermind-actual-model", providerResp.Model)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytesOut)

	mlog.Info("v1.messages.request",
		mlog.String("request_model", requestModel),
		mlog.String("actual_model", providerResp.Model),
		mlog.Bool("stream", false),
		mlog.Int("input_tokens", providerResp.Usage.PromptTokens),
		mlog.Int("output_tokens", providerResp.Usage.CompletionTokens),
		mlog.Int64("duration_ms", time.Since(start).Milliseconds()),
		mlog.Int("status", http.StatusOK))
}

// serveV1MessagesStream is the streaming branch (Task 7).
func (s *Server) serveV1MessagesStream(
	w http.ResponseWriter, r *http.Request,
	p core.LanguageModel, req *provider.Request, requestModel string, start time.Time,
) {
	pantheonReq := providerRequestToPantheon(req)
	stream, err := p.Stream(r.Context(), pantheonReq)
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
		mlog.Warning("v1.messages.stream_error", mlog.String("err", err.Error()))
	}
	mlog.Info("v1.messages.request",
		mlog.String("request_model", requestModel),
		mlog.Bool("stream", true),
		mlog.Int64("duration_ms", time.Since(start).Milliseconds()),
		mlog.Int("status", http.StatusOK))
}

func providerRequestToPantheon(req *provider.Request) *core.Request {
	if req == nil {
		return nil
	}
	msgs := make([]core.Message, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = message.ToPantheon(message.LegacyToHermindMessage(m))
	}
	pReq := &core.Request{
		Messages:     msgs,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
	}
	if req.MaxTokens > 0 {
		pReq.MaxTokens = &req.MaxTokens
	}
	if req.Temperature != nil {
		pReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		pReq.TopP = req.TopP
	}
	if len(req.StopSequences) > 0 {
		pReq.StopSequences = req.StopSequences
	}
	if len(req.Tools) > 0 {
		pReq.ToolChoice = core.ToolChoice{Mode: core.ToolChoiceModeAuto}
	}
	return pReq
}

func pantheonResponseToProvider(resp *core.Response) *provider.Response {
	if resp == nil {
		return nil
	}
	msg := message.HermindMessageToLegacy(message.MessageFromPantheon(resp.Message))
	return &provider.Response{
		Message:      msg,
		FinishReason: resp.FinishReason,
		Usage: message.Usage{
			InputTokens:      resp.Usage.PromptTokens,
			OutputTokens:     resp.Usage.CompletionTokens,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		},
		Model: resp.Model,
	}
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
	mlog.Warning("v1.messages.provider_error", mlog.String("err", err.Error()))
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
