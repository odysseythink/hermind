package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// GenerateToken returns a random URL-safe token suitable for a
// single server-boot session. 32 bytes -> 43 base64url chars.
func GenerateToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// NewAuthMiddleware returns a middleware that enforces a Bearer token
// on incoming requests, bypassing the allowlist of public paths and
// accepting the token via either the Authorization header or a
// "t" query parameter (used for embedding the token in the served
// landing page URL).
func NewAuthMiddleware(token string, publicPaths []string) func(http.Handler) http.Handler {
	publicSet := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		publicSet[p] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := publicSet[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			if checkToken(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

func checkToken(r *http.Request, token string) bool {
	const prefix = "Bearer "
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, prefix) {
		got := auth[len(prefix):]
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1 {
			return true
		}
	}
	if q := r.URL.Query().Get("t"); q != "" {
		if subtle.ConstantTimeCompare([]byte(q), []byte(token)) == 1 {
			return true
		}
	}
	return false
}
