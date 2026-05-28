package tools

import (
	"encoding/json"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const gmailConfigKey = "gmail_agent_config"

type gmailBridgeConfig struct {
	DeploymentID string `json:"deploymentId"`
	APIKey       string `json:"apiKey"`
}

func parseGmailBridgeConfig(raw string) (gmailBridgeConfig, bool) {
	if raw == "" {
		return gmailBridgeConfig{}, false
	}
	var c gmailBridgeConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, false
	}
	return c, c.DeploymentID != "" && c.APIKey != ""
}

var gmailDestructiveActions = map[string]bool{
	"create_draft":    true,
	"update_draft":    true,
	"send_draft":      true,
	"send_email":      true,
	"reply_to_thread": true,
	"delete_draft":    true,
	"move_to_trash":   true,
}

func NewGmailAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
	e := newBridgeBackedSkill("gmail-agent", "gmail", gmailConfigKey,
		"Search, read, draft, and send Gmail messages via a Google Apps Script bridge configured by the admin.",
		gmailAgentSchema(), gmailDestructiveActions, tc, deps)
	e.Emoji = "✉️"
	e.MaxResultChars = 16 * 1024
	return e
}

func gmailAgentSchema() *core.Schema {
	s, _ := core.SchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["search","read_thread","list_drafts","get_draft","mailbox_stats","create_draft","update_draft","send_draft","send_email","reply_to_thread","delete_draft","move_to_trash"]},
			"query": {"type": "string"},
			"thread_id": {"type": "string"},
			"draft_id": {"type": "string"},
			"to": {"type": "string"},
			"subject": {"type": "string"},
			"body": {"type": "string"},
			"label": {"type": "string"}
		},
		"required": ["action"]
	}`))
	return s
}
