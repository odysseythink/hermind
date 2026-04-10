// provider/anthropic/errors.go
package anthropic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nousresearch/hermes-agent/provider"
)

// mapHTTPError converts an Anthropic HTTP error response to a provider.Error.
func mapHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var apiErr apiErrorResponse
	_ = json.Unmarshal(body, &apiErr)

	msg := apiErr.Error.Message
	if msg == "" {
		msg = fmt.Sprintf("anthropic http %d: %s", resp.StatusCode, string(body))
	}

	kind := provider.ErrUnknown
	switch resp.StatusCode {
	case http.StatusTooManyRequests: // 429
		kind = provider.ErrRateLimit
	case http.StatusUnauthorized, http.StatusForbidden: // 401, 403
		kind = provider.ErrAuth
	case http.StatusBadRequest: // 400
		// Anthropic returns 400 for both invalid requests AND context-too-long
		if apiErr.Error.Type == "invalid_request_error" &&
			containsContextTooLong(msg) {
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
		Provider:   "anthropic",
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

// containsContextTooLong checks if an error message indicates context overflow.
func containsContextTooLong(msg string) bool {
	for _, needle := range []string{"context length", "too long", "maximum context", "context window"} {
		if containsIgnoreCase(msg, needle) {
			return true
		}
	}
	return false
}

// containsIgnoreCase is a tiny case-insensitive substring check.
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
