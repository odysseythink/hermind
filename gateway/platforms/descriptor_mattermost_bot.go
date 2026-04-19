package platforms

import "github.com/odysseythink/hermind/gateway"

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
	})
}
