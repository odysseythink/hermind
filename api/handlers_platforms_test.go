package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/api"
	"github.com/odysseythink/hermind/config"
)

func TestPlatformsSchema_ContainsAllRegisteredTypes(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.PlatformsSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{
		"api_server", "acp", "webhook",
		"telegram", "discord", "discord_bot",
		"slack", "slack_events",
		"mattermost", "mattermost_bot",
		"feishu", "dingtalk", "wecom",
		"matrix", "signal", "whatsapp",
		"homeassistant", "email", "sms",
	}
	have := map[string]bool{}
	for _, d := range body.Descriptors {
		have[d.Type] = true
	}
	for _, t0 := range want {
		if !have[t0] {
			t.Errorf("missing descriptor: %q", t0)
		}
	}
}

func TestPlatformsSchema_TelegramFieldShape(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var body api.PlatformsSchemaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)

	var tg *api.SchemaDescriptorDTO
	for i := range body.Descriptors {
		if body.Descriptors[i].Type == "telegram" {
			tg = &body.Descriptors[i]
			break
		}
	}
	if tg == nil {
		t.Fatal("telegram descriptor not in response")
	}
	if len(tg.Fields) != 2 {
		t.Fatalf("telegram fields = %d, want 2", len(tg.Fields))
	}
	var tokenField, proxyField *api.SchemaFieldDTO
	for i := range tg.Fields {
		switch tg.Fields[i].Name {
		case "token":
			tokenField = &tg.Fields[i]
		case "proxy":
			proxyField = &tg.Fields[i]
		}
	}
	if tokenField == nil {
		t.Fatal("telegram descriptor missing token field")
	}
	if tokenField.Kind != "secret" {
		t.Errorf("token kind = %q, want secret", tokenField.Kind)
	}
	if !tokenField.Required {
		t.Errorf("token.Required = false, want true")
	}
	if proxyField == nil {
		t.Fatal("telegram descriptor missing proxy field")
	}
	if proxyField.Kind != "string" {
		t.Errorf("proxy kind = %q, want string", proxyField.Kind)
	}
	if proxyField.Required {
		t.Errorf("proxy.Required = true, want false")
	}
	if proxyField.Help == "" {
		t.Errorf("proxy.Help is empty, want help text")
	}
}

func TestPlatformsSchema_RequiresAuth(t *testing.T) {
	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/platforms/schema", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestPlatformReveal_ReturnsSecretValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"tg_main": {
			Enabled: true, Type: "telegram",
			Options: map[string]string{"token": "live-token"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/tg_main/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.RevealResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if body.Value != "live-token" {
		t.Errorf("value = %q, want %q", body.Value, "live-token")
	}
}

func TestPlatformReveal_RejectsNonSecretField(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Platforms = map[string]config.PlatformConfig{
		"slack_ops": {
			Enabled: true, Type: "slack_events",
			Options: map[string]string{"addr": ":9000", "bot_token": "xoxb-y"},
		},
	}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/slack_ops/reveal", strings.NewReader(`{"field":"addr"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPlatformReveal_404OnUnknownKey(t *testing.T) {
	cfg := &config.Config{}
	srv, _ := api.NewServer(&api.ServerOpts{Config: cfg, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/missing/reveal", strings.NewReader(`{"field":"token"}`))
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

type stubController struct {
	testErr    error
	applyCalls int
	applyRes   api.ApplyResult
	applyErr   error
}

func (s *stubController) TestPlatform(_ context.Context, _ string) error {
	return s.testErr
}
func (s *stubController) Apply(_ context.Context) (api.ApplyResult, error) {
	s.applyCalls++
	return s.applyRes, s.applyErr
}

func TestPlatformTest_NilControllerReturns503(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{Config: &config.Config{}, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestPlatformTest_NotImplementedReturns501(t *testing.T) {
	ctrl := &stubController{testErr: api.ErrTestNotImplemented}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", w.Code)
	}
}

func TestPlatformTest_SuccessReturnsOK(t *testing.T) {
	ctrl := &stubController{testErr: nil}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.PlatformTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !body.OK {
		t.Errorf("body.OK = false, error = %q", body.Error)
	}
}

func TestPlatformTest_FailureReturnsOKFalse(t *testing.T) {
	ctrl := &stubController{testErr: errors.New("auth failed: bad token")}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/any/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.PlatformTestResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.OK {
		t.Error("body.OK = true, want false")
	}
	if !strings.Contains(body.Error, "auth failed") {
		t.Errorf("body.Error = %q, want substring 'auth failed'", body.Error)
	}
}

func TestPlatformsApply_NilControllerReturns503(t *testing.T) {
	srv, _ := api.NewServer(&api.ServerOpts{Config: &config.Config{}, Token: "test-token"})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestPlatformsApply_SuccessReturnsPayload(t *testing.T) {
	ctrl := &stubController{
		applyRes: api.ApplyResult{OK: true, Restarted: []string{"tg_main"}, TookMS: 42},
	}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.ApplyResult
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if !body.OK || len(body.Restarted) != 1 || body.Restarted[0] != "tg_main" {
		t.Errorf("body = %+v", body)
	}
	if ctrl.applyCalls != 1 {
		t.Errorf("applyCalls = %d, want 1", ctrl.applyCalls)
	}
}

func TestPlatformsApply_ConcurrentReturns409(t *testing.T) {
	ctrl := &stubController{applyErr: api.ErrApplyInProgress}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestPlatformsApply_GenericErrorReturnsOKFalse(t *testing.T) {
	ctrl := &stubController{applyErr: errors.New("rebuild failed: config parse error")}
	srv, _ := api.NewServer(&api.ServerOpts{
		Config: &config.Config{}, Token: "test-token", Controller: ctrl,
	})
	req := httptest.NewRequest("POST", "/api/platforms/apply", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body api.ApplyResult
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.OK {
		t.Error("body.OK = true, want false")
	}
	if !strings.Contains(body.Error, "rebuild failed") {
		t.Errorf("body.Error = %q", body.Error)
	}
}
