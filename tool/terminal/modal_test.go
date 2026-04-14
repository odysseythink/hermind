// tool/terminal/modal_test.go
package terminal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModalRequiresBaseURL(t *testing.T) {
	_, err := NewModal(Config{ModalToken: "t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modal_base_url")
}

func TestNewModalRequiresToken(t *testing.T) {
	_, err := NewModal(Config{ModalBaseURL: "https://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "modal_token")
}

func TestModalExecuteHappyPath(t *testing.T) {
	var captured modalExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/exec", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := modalExecResponse{
			Stdout:     "hello\n",
			Stderr:     "",
			ExitCode:   0,
			DurationMS: 42,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m, err := NewModal(Config{ModalBaseURL: srv.URL, ModalToken: "test-token"})
	require.NoError(t, err)

	result, err := m.Execute(context.Background(), "echo hello", &ExecOptions{
		Cwd:     "/workspace",
		Env:     map[string]string{"FOO": "bar"},
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "echo hello", captured.Command)
	assert.Equal(t, "/workspace", captured.Cwd)
	assert.Equal(t, "bar", captured.Env["FOO"])
	assert.Equal(t, 30, captured.TimeoutSeconds)
}

func TestModalExecuteHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m, _ := NewModal(Config{ModalBaseURL: srv.URL, ModalToken: "t"})
	_, err := m.Execute(context.Background(), "echo hi", &ExecOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 500")
}

func TestModalSupportsPersistentShellIsFalse(t *testing.T) {
	m, _ := NewModal(Config{ModalBaseURL: "https://x", ModalToken: "t"})
	assert.False(t, m.SupportsPersistentShell())
	assert.NoError(t, m.Close())
}
