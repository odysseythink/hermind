package tools

import (
	"encoding/json"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const gcalConfigKey = "google_calendar_agent_config"

var gcalDestructiveActions = map[string]bool{
	"quick_add":    true,
	"create_event": true,
	"update_event": true,
}

func NewGCalAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
	e := newBridgeBackedSkill("google-calendar-agent", "google-calendar", gcalConfigKey,
		"List calendars, query events, and create/update events via a Google Apps Script bridge.",
		gcalAgentSchema(), gcalDestructiveActions, tc, deps)
	e.Emoji = "📅"
	e.MaxResultChars = 12 * 1024
	return e
}

func gcalAgentSchema() *core.Schema {
	s, _ := core.SchemaFromJSON(json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["list_calendars","get_calendar","get_event","get_events_for_day","get_events","quick_add","create_event","update_event"]},
			"calendar_id": {"type": "string"},
			"event_id": {"type": "string"},
			"date": {"type": "string"},
			"title": {"type": "string"},
			"description": {"type": "string"},
			"start_time": {"type": "string"},
			"end_time": {"type": "string"},
			"text": {"type": "string"}
		},
		"required": ["action"]
	}`))
	return s
}
