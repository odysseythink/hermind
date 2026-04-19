package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_AllowsPublicPath(t *testing.T) {
	mw := NewAuthMiddleware("secret", []string{"/api/status"})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_RejectsMissingToken(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not call") })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_AcceptsValidBearer(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_AcceptsQueryParamToken(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })

	req := httptest.NewRequest("GET", "/api/sessions?t=secret", nil)
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	mw := NewAuthMiddleware("secret", nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not call") })

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Errorf("code = %d", rr.Code)
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || b == "" {
		t.Error("empty token")
	}
	if a == b {
		t.Error("tokens repeated")
	}
	if len(a) < 32 {
		t.Errorf("token too short: %d chars", len(a))
	}
}
