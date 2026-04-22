package api

import (
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// sessionIDParam returns the {id} path parameter with any percent-encoding
// resolved. chi does not auto-decode URL params, so a URL like
// /api/sessions/telegram%3A760061130 would otherwise surface as
// "telegram%3A760061130" and never match the stored session id
// "telegram:760061130". Any handler that looks a session up by its path
// parameter MUST use this helper.
//
// On decode failure the raw value is returned so callers still see a
// non-empty id and can fall through to the normal "not found" path.
func sessionIDParam(r *http.Request) string {
	raw := chi.URLParam(r, "id")
	if raw == "" {
		return ""
	}
	if dec, err := url.PathUnescape(raw); err == nil {
		return dec
	}
	return raw
}
