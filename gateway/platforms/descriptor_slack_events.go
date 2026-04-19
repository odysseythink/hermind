package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "slack_events",
		DisplayName: "Slack (Events API)",
		Summary:     "Bidirectional Slack integration via the Events API + chat.postMessage.",
		Fields: []FieldSpec{
			{Name: "addr", Label: "Listen Address", Kind: FieldString, Required: true,
				Help: `e.g. ":8082". Must match the URL Slack posts events to.`},
			{Name: "bot_token", Label: "Bot Token", Kind: FieldSecret, Required: true,
				Help: `"xoxb-..." — from Slack app OAuth settings.`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewSlackEvents(opts["addr"], opts["bot_token"]), nil
		},
	})
}
