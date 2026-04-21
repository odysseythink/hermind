package platforms

import (
	"strings"
	"testing"
)

func TestFeishuApp_MissingCreds(t *testing.T) {
	cases := []struct {
		name string
		opts map[string]string
	}{
		{"no app_id", map[string]string{"app_secret": "s"}},
		{"no app_secret", map[string]string{"app_id": "a"}},
		{"both empty", map[string]string{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fa, err := NewFeishuApp(tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if fa != nil {
				t.Errorf("expected nil FeishuApp, got %v", fa)
			}
			if !strings.Contains(err.Error(), "app_id") && !strings.Contains(err.Error(), "app_secret") {
				t.Errorf("error should mention app_id or app_secret: %v", err)
			}
		})
	}
}

func TestFeishuApp_WebhookURLSurfaced(t *testing.T) {
	fa, err := NewFeishuApp(map[string]string{
		"webhook_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx",
	})
	if err == nil {
		t.Fatal("expected migration error, got nil")
	}
	if fa != nil {
		t.Errorf("expected nil FeishuApp on migration error, got %v", fa)
	}
	if !strings.Contains(err.Error(), "webhook_url is no longer supported") {
		t.Errorf("error should mention migration: %v", err)
	}
}
