// provider/openaicompat/errors.go
package openaicompat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/odysseythink/hermind/provider"
)

// mapHTTPError converts an OpenAI-compatible error response to a provider.Error.
// Takes the provider name separately so wrapper providers attribute correctly.
func mapHTTPError(providerName string, resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)

	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("%s http %d: %s", providerName, resp.StatusCode, string(body))
	}

	kind := provider.ErrUnknown
	switch resp.StatusCode {
	case http.StatusTooManyRequests: // 429
		kind = provider.ErrRateLimit
	case http.StatusUnauthorized, http.StatusForbidden: // 401, 403
		kind = provider.ErrAuth
	case http.StatusBadRequest: // 400
		if isContextTooLong(msg) || isContextTooLong(apiErr.Error.Code) {
			kind = provider.ErrContextTooLong
		} else {
			kind = provider.ErrInvalidRequest
		}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout: // 408, 504
		kind = provider.ErrTimeout
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable: // 500, 502, 503
		kind = provider.ErrServerError
	}

	return &provider.Error{
		Kind:       kind,
		Provider:   providerName,
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

// isContextTooLong checks for common phrases signaling context window overflow.
// OpenAI uses "maximum context length", DeepSeek uses "context length exceeded", etc.
func isContextTooLong(s string) bool {
	l := strings.ToLower(s)
	for _, needle := range []string{
		"maximum context",
		"context length",
		"context window",
		"token limit",
		"context_length_exceeded",
		"string_above_max_length",
	} {
		if strings.Contains(l, needle) {
			return true
		}
	}
	return false
}
