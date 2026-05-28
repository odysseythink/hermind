package flow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newFlowContext() *Context {
	return &Context{
		Variables:       map[string]string{},
		Emit:            func(string) {},
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
		AllowPrivateIPs: true, // allow httptest servers in tests
	}
}

func TestAPICall_GET_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":    srv.URL + "/data",
		"method": "GET",
	})
	require.NoError(t, err)
	require.Equal(t, "ok", out)
}

func TestAPICall_POST_JSONBody(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte("created"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":      srv.URL,
		"method":   "POST",
		"bodyType": "json",
		"body":     `{"name":"alice"}`,
	})
	require.NoError(t, err)
	require.Equal(t, "created", out)
	require.Equal(t, "alice", captured["name"])
}

func TestAPICall_POST_FormBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		r.ParseForm()
		require.Equal(t, "v1", r.FormValue("key1"))
		w.Write([]byte("form-ok"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":      srv.URL,
		"method":   "POST",
		"bodyType": "form",
		"formData": []any{
			map[string]any{"key": "key1", "value": "v1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "form-ok", out)
}

func TestAPICall_POST_TextBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		require.Equal(t, "plain text", string(body))
		w.Write([]byte("text-ok"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":      srv.URL,
		"method":   "POST",
		"bodyType": "text",
		"body":     "plain text",
	})
	require.NoError(t, err)
	require.Equal(t, "text-ok", out)
}

func TestAPICall_HeadersForwarded(t *testing.T) {
	var headerVal string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerVal = r.Header.Get("X-Custom")
		w.Write([]byte("hdr"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	_, _ = ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":     srv.URL,
		"headers": []any{map[string]any{"key": "X-Custom", "value": "my-val"}},
	})
	require.Equal(t, "my-val", headerVal)
}

func TestAPICall_VarInterpolation_URL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/users/42", r.URL.Path)
		w.Write([]byte("user-42"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	fc.Variables["userId"] = "42"
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url": srv.URL + "/users/{{userId}}",
	})
	require.NoError(t, err)
	require.Equal(t, "user-42", out)
}

func TestAPICall_VarInterpolation_Body(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	fc.Variables["name"] = "bob"
	_, _ = ExecuteAPICall(context.Background(), fc, map[string]any{
		"url":      srv.URL,
		"bodyType": "json",
		"body":     `{"name":"{{name}}"}`,
	})
	require.Equal(t, "bob", captured["name"])
}

func TestAPICall_HTTPError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	_, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url": srv.URL,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 400")
}

func TestAPICall_SSRFGuardBlocks(t *testing.T) {
	fc := &Context{
		Variables:       map[string]string{},
		Emit:            func(string) {},
		HTTPClient:      &http.Client{Timeout: 5 * time.Second},
		AllowPrivateIPs: false,
	}
	_, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url": "http://127.0.0.1/secret",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "private IP")
}

func TestAPICall_ResponseTextReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response body"))
	}))
	defer srv.Close()

	fc := newFlowContext()
	out, err := ExecuteAPICall(context.Background(), fc, map[string]any{
		"url": srv.URL,
	})
	require.NoError(t, err)
	require.Equal(t, "response body", out)
}
