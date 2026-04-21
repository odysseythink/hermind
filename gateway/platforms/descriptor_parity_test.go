package platforms

import (
	"testing"
)

// parityCases is the canonical list of (type, minimal valid options)
// pairs copied from the old cli/gateway.go::buildPlatform switch. Every
// case must resolve to a registered descriptor that builds without
// error. If this test drifts from buildPlatform, either the registry is
// missing a type (tasks 3–5 cover all 19) or buildPlatform grew a new
// one that needs a descriptor.
var parityCases = []struct {
	Type    string
	Options map[string]string
}{
	{"api_server", map[string]string{"addr": ":9000"}},
	{"webhook", map[string]string{"url": "https://example.com/hook", "token": "t"}},
	{"telegram", map[string]string{"token": "123:abc"}},
	{"acp", map[string]string{"addr": ":9001", "token": "t"}},
	{"slack", map[string]string{"webhook_url": "https://hooks.slack.com/xxx"}},
	{"discord", map[string]string{"webhook_url": "https://discord.com/api/webhooks/xxx"}},
	{"mattermost", map[string]string{"webhook_url": "https://mm.example.com/hooks/xxx"}},
	{"feishu", map[string]string{"app_id": "a", "app_secret": "s", "domain": "feishu"}},
	{"dingtalk", map[string]string{"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx"}},
	{"wecom", map[string]string{"webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"}},
	{"email", map[string]string{
		"host": "smtp.example.com", "port": "587",
		"username": "u", "password": "p",
		"from": "from@example.com", "to": "to@example.com",
	}},
	// sms: Twilio-shaped options (account_sid / auth_token).
	{"sms", map[string]string{
		"account_sid": "ACxxx", "auth_token": "token",
		"from": "+10000000000", "to": "+10000000001",
	}},
	{"signal", map[string]string{"base_url": "http://localhost:8080", "account": "+10000000000"}},
	{"whatsapp", map[string]string{"phone_id": "123", "access_token": "t"}},
	{"matrix", map[string]string{
		"home_server": "https://matrix.org", "access_token": "t", "room_id": "!room:matrix.org",
	}},
	{"homeassistant", map[string]string{
		"base_url": "http://homeassistant.local:8123", "access_token": "t", "service": "notify",
	}},
	{"slack_events", map[string]string{"addr": ":8082", "bot_token": "xoxb-xxx"}},
	{"discord_bot", map[string]string{"token": "xxx", "channel_id": "1"}},
	{"mattermost_bot", map[string]string{"base_url": "https://mm.example.com", "token": "t", "channel_id": "c"}},
}

func TestDescriptorParity_AllTypesRegistered(t *testing.T) {
	for _, tc := range parityCases {
		tc := tc
		t.Run(tc.Type, func(t *testing.T) {
			d, ok := Get(tc.Type)
			if !ok {
				t.Fatalf("no descriptor registered for %q", tc.Type)
			}
			if d.Build == nil {
				t.Fatalf("%q: Build is nil", tc.Type)
			}
			plat, err := d.Build(tc.Options)
			if err != nil {
				t.Fatalf("%q: Build returned error: %v", tc.Type, err)
			}
			if plat == nil {
				t.Fatalf("%q: Build returned nil platform", tc.Type)
			}
			if plat.Name() == "" {
				t.Errorf("%q: platform has empty Name()", tc.Type)
			}
		})
	}
}

func TestDescriptorParity_CoverageMatchesCaseCount(t *testing.T) {
	// Any drift from 19 is intentional and should update parityCases
	// plus the descriptor files together.
	if want, got := 19, len(parityCases); got != want {
		t.Fatalf("parityCases has %d entries, want %d — update parityCases and descriptors in lockstep", got, want)
	}
}

// TestDescriptorParity_NoUnknownTypes is the complement to
// CoverageMatchesCaseCount: any type that self-registers must have a
// matching entry in parityCases. Catches the drift where someone adds a
// 20th descriptor but forgets to update the guard slice.
func TestDescriptorParity_NoUnknownTypes(t *testing.T) {
	known := map[string]bool{}
	for _, tc := range parityCases {
		known[tc.Type] = true
	}
	for _, d := range All() {
		if !known[d.Type] {
			t.Errorf("registered descriptor %q has no entry in parityCases — add it or remove the descriptor", d.Type)
		}
	}
}
