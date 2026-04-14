package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
)

func TestRegisterAllAndDispatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/sessions" && r.Method == "POST":
			_, _ = w.Write([]byte(`{"id":"sess_42","connectUrl":"wss://c/sess_42"}`))
		case strings.HasSuffix(r.URL.Path, "/debug") && r.Method == "GET":
			_, _ = w.Write([]byte(`{"debuggerFullscreenUrl":"https://live/sess_42"}`))
		case r.URL.Path == "/v1/sessions/sess_42" && r.Method == "POST":
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("BROWSERBASE_API_KEY", "k")
	t.Setenv("BROWSERBASE_PROJECT_ID", "proj")
	p := NewBrowserbase(config.BrowserbaseConfig{BaseURL: srv.URL})

	reg := tool.NewRegistry()
	RegisterAll(reg, p)

	ctx := context.Background()
	res, err := reg.Dispatch(ctx, "browser_session_create", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("create dispatch: %v", err)
	}
	if !strings.Contains(res, "sess_42") {
		t.Errorf("missing session id in create result: %s", res)
	}

	closeArgs, _ := json.Marshal(map[string]string{"session_id": "sess_42"})
	res, err = reg.Dispatch(ctx, "browser_session_close", closeArgs)
	if err != nil {
		t.Fatalf("close dispatch: %v", err)
	}
	if !strings.Contains(res, `"ok":true`) {
		t.Errorf("expected ok:true, got %s", res)
	}
}

func TestRegisterAllSkipsUnconfigured(t *testing.T) {
	t.Setenv("BROWSERBASE_API_KEY", "")
	t.Setenv("BROWSERBASE_PROJECT_ID", "")
	reg := tool.NewRegistry()
	p := NewBrowserbase(config.BrowserbaseConfig{})
	RegisterAll(reg, p)
	if len(reg.Definitions(nil)) != 0 {
		t.Errorf("expected no tools registered when not configured")
	}
}
