package platforms

import (
	"context"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/gateway"
)

func init() {
	Register(Descriptor{
		Type:        "mattermost_bot",
		DisplayName: "Mattermost Bot (REST poll)",
		Summary:     "Polls a single channel for mentions and replies via the REST API.",
		Fields: []FieldSpec{
			{Name: "base_url", Label: "Server Base URL", Kind: FieldString, Required: true,
				Help: `e.g. "https://mm.example.com".`},
			{Name: "token", Label: "Personal Access Token", Kind: FieldSecret, Required: true},
			{Name: "channel_id", Label: "Channel ID", Kind: FieldString, Required: true},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMattermostBot(opts["base_url"], opts["token"], opts["channel_id"]), nil
		},
		Test: func(ctx context.Context, opts map[string]string) error {
			return testMattermostBot(ctx, opts["base_url"], opts["token"])
		},
	})
}

func testMattermostBot(ctx context.Context, baseURL, token string) error {
	if baseURL == "" || token == "" {
		return fmt.Errorf("mattermost_bot: base_url and token are required")
	}
	base := strings.TrimRight(baseURL, "/")
	return httpProbe(ctx, "GET", base+"/api/v4/users/me", map[string]string{
		"Authorization": "Bearer " + token,
	})
}
