// tool/terminal/daytona_test.go
package terminal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDaytonaRequiresBaseURL(t *testing.T) {
	_, err := NewDaytona(Config{DaytonaToken: "t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daytona_base_url")
}

func TestNewDaytonaRequiresToken(t *testing.T) {
	_, err := NewDaytona(Config{DaytonaBaseURL: "https://x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "daytona_token")
}

func TestDaytonaExecuteHappyPath(t *testing.T) {
	var captured daytonaExecRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/workspace/exec", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &captured))

		resp := daytonaExecResponse{
			Stdout:     "world\n",
			Stderr:     "",
			ExitCode:   0,
			DurationMS: 55,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	d, err := NewDaytona(Config{DaytonaBaseURL: srv.URL, DaytonaToken: "test-token"})
	require.NoError(t, err)

	result, err := d.Execute(context.Background(), "echo world", &ExecOptions{})
	require.NoError(t, err)
	assert.Equal(t, "world\n", result.Stdout)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "echo world", captured.Command)
}

func TestDaytonaExecuteHandlesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	d, _ := NewDaytona(Config{DaytonaBaseURL: srv.URL, DaytonaToken: "t"})
	_, err := d.Execute(context.Background(), "echo hi", &ExecOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http 502")
}

func TestDaytonaSupportsPersistentShellIsFalse(t *testing.T) {
	d, _ := NewDaytona(Config{DaytonaBaseURL: "https://x", DaytonaToken: "t"})
	assert.False(t, d.SupportsPersistentShell())
	assert.NoError(t, d.Close())
}
