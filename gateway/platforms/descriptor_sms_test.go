package platforms

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSMS_Success(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"sid":"ACabc","status":"active"}`))
	}))
	defer srv.Close()

	if err := testSMS(context.Background(), "ACabc", "tok", srv.URL); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotPath != "/2010-04-01/Accounts/ACabc.json" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic …", gotAuth)
	}
	decoded, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(gotAuth, "Basic "))
	if string(decoded) != "ACabc:tok" {
		t.Errorf("decoded creds = %q, want %q", decoded, "ACabc:tok")
	}
}

func TestSMS_MissingCreds(t *testing.T) {
	if err := testSMS(context.Background(), "", "tok", "http://unused"); err == nil {
		t.Error("expected error for empty sid")
	}
	if err := testSMS(context.Background(), "AC", "", "http://unused"); err == nil {
		t.Error("expected error for empty token")
	}
}
