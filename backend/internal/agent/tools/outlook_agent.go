package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	oauthpkg "github.com/odysseythink/hermind/backend/internal/agent/tools/oauth"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

const (
	outlookConfigKey = "outlook_agent_config"
	graphAPIBase     = "https://graph.microsoft.com/v1.0"
)

var (
	// testGraphBase overrides graph.microsoft.com during tests.
	testGraphBase             string
	outlookDestructiveActions = map[string]bool{
		"create_draft": true, "send_email": true,
		"update_draft": true, "delete_draft": true, "send_draft": true,
		"create_draft_reply": true, "reply_to_message": true,
		"mark_read": true, "mark_unread": true,
	}
)

func SetTestGraphBase(u string) { testGraphBase = u }

type outlookSkillConfig struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Tenant       string `json:"tenant,omitempty"`
}

func NewOutlookAgentSkill(tc *ToolContext, deps BuilderDeps) *tool.Entry {
	return &tool.Entry{
		Name:           "outlook-agent",
		Toolset:        "outlook",
		Description:    "Search, read, draft, and send Outlook messages via Microsoft Graph (single-user OAuth).",
		Emoji:          "📧",
		MaxResultChars: 16 * 1024,
		CheckFn: func() bool {
			if deps.Cfg == nil {
				return false
			}
			if deps.OutlookOAuth == nil || deps.OutlookStore == nil {
				return false
			}
			cfg, ok := parseOutlookSkillConfig(tc.Settings[outlookConfigKey])
			if !ok {
				return false
			}
			_ = cfg
			if tc.User == nil {
				return false
			}
			_, err := deps.OutlookStore.Get(tc.Ctx, tc.User.ID)
			return err == nil
		},
		Schema: core.ToolDefinition{
			Name:        "outlook-agent",
			Description: "Outlook mail operations",
			Parameters:  outlookAgentSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args map[string]any
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			action, _ := args["action"].(string)
			if action == "" {
				return tool.Error("action is required"), nil
			}

			cfg, ok := parseOutlookSkillConfig(tc.Settings[outlookConfigKey])
			if !ok {
				return tool.Error("outlook not configured"), nil
			}

			if outlookDestructiveActions[action] && tc.Approval != nil {
				desc := fmt.Sprintf("Outlook %s", action)
				if to, _ := args["to"].(string); to != "" {
					desc += " to " + to
				}
				if ok, reason := tc.Approval(ctx, "outlook-agent:"+action, args, desc); !ok {
					return tool.Error("rejected: " + reason), nil
				}
			}

			// Parse attachments and inline their text into the body.
			if rawAtts, ok := args["attachments"].([]any); ok && len(rawAtts) > 0 {
				if deps.Collector != nil {
					var atts []oauthpkg.Attachment
					for _, item := range rawAtts {
						blob, _ := json.Marshal(item)
						var a oauthpkg.Attachment
						_ = json.Unmarshal(blob, &a)
						atts = append(atts, a)
					}
					attText, err := oauthpkg.ParseAttachments(ctx, deps.Collector, atts)
					if err != nil {
						return tool.Error("attachment parse: " + err.Error()), nil
					}
					if attText != "" {
						body, _ := args["body"].(string)
						args["body"] = body + attText
					}
				}
			}

			if tc.User == nil {
				return tool.Error("user required for outlook"), nil
			}
			tok, err := deps.OutlookOAuth.ValidAccessToken(ctx, tc.User.ID, cfg.ClientID, cfg.ClientSecret)
			if err != nil {
				return tool.Error("token refresh: " + err.Error()), nil
			}

			tc.Emit("Outlook: " + action)

			switch action {
			case "search":
				q, _ := args["query"].(string)
				v := url.Values{}
				v.Set("$search", `"`+q+`"`)
				v.Set("$top", "10")
				return graphGET(ctx, tok, "/me/messages?"+v.Encode())
			case "read_thread":
				cid, _ := args["conversation_id"].(string)
				v := url.Values{}
				v.Set("$filter", "conversationId eq '"+cid+"'")
				return graphGET(ctx, tok, "/me/messages?"+v.Encode())
			case "read_message":
				mid, _ := args["message_id"].(string)
				return graphGET(ctx, tok, "/me/messages/"+mid)
			case "create_draft":
				return graphPOST(ctx, tok, "/me/messages", buildOutlookMessage(args))
			case "send_email":
				return graphPOST(ctx, tok, "/me/sendMail", map[string]any{"message": buildOutlookMessage(args)})
			case "get_inbox":
				limit := intArg(args, "limit", 25)
				return graphGET(ctx, tok, fmt.Sprintf("/me/mailFolders/inbox/messages?$top=%d", limit))
			case "list_drafts":
				limit := intArg(args, "limit", 25)
				return graphGET(ctx, tok, fmt.Sprintf("/me/mailFolders/drafts/messages?$top=%d", limit))
			case "get_draft":
				id, _ := args["draft_id"].(string)
				if id == "" {
					return tool.Error("draft_id required"), nil
				}
				return graphGET(ctx, tok, "/me/messages/"+id)
			case "update_draft":
				id, _ := args["draft_id"].(string)
				if id == "" {
					return tool.Error("draft_id required"), nil
				}
				return graphPATCH(ctx, tok, "/me/messages/"+id, buildOutlookMessage(args))
			case "delete_draft":
				id, _ := args["draft_id"].(string)
				if id == "" {
					return tool.Error("draft_id required"), nil
				}
				return graphDELETE(ctx, tok, "/me/messages/"+id)
			case "send_draft":
				id, _ := args["draft_id"].(string)
				if id == "" {
					return tool.Error("draft_id required"), nil
				}
				return graphPOST(ctx, tok, "/me/messages/"+id+"/send", nil)
			case "create_draft_reply":
				id, _ := args["message_id"].(string)
				if id == "" {
					return tool.Error("message_id required"), nil
				}
				return graphPOST(ctx, tok, "/me/messages/"+id+"/createReply", map[string]any{
					"comment": args["body"],
				})
			case "reply_to_message":
				id, _ := args["message_id"].(string)
				if id == "" {
					return tool.Error("message_id required"), nil
				}
				return graphPOST(ctx, tok, "/me/messages/"+id+"/reply", map[string]any{
					"comment": args["body"],
				})
			case "mark_read":
				id, _ := args["message_id"].(string)
				if id == "" {
					return tool.Error("message_id required"), nil
				}
				return graphPATCH(ctx, tok, "/me/messages/"+id, map[string]any{"isRead": true})
			case "mark_unread":
				id, _ := args["message_id"].(string)
				if id == "" {
					return tool.Error("message_id required"), nil
				}
				return graphPATCH(ctx, tok, "/me/messages/"+id, map[string]any{"isRead": false})
			default:
				return tool.Error("unknown action: " + action), nil
			}
		},
	}
}

func buildOutlookMessage(args map[string]any) map[string]any {
	to, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)
	return map[string]any{
		"subject": subject,
		"body":    map[string]any{"contentType": "Text", "content": body},
		"toRecipients": []map[string]any{
			{"emailAddress": map[string]any{"address": to}},
		},
	}
}

func parseOutlookSkillConfig(raw string) (outlookSkillConfig, bool) {
	if raw == "" {
		return outlookSkillConfig{}, false
	}
	var c outlookSkillConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, false
	}
	return c, c.ClientID != "" && c.ClientSecret != ""
}

func outlookAgentSchema() *core.Schema {
	s, _ := core.SchemaFromJSON(json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["search","read_thread","read_message","create_draft","send_email","get_inbox","list_drafts","get_draft","update_draft","delete_draft","send_draft","create_draft_reply","reply_to_message","mark_read","mark_unread"]},
			"query": {"type": "string"},
			"conversation_id": {"type": "string"},
			"message_id": {"type": "string"},
			"to": {"type": "string"},
			"subject": {"type": "string"},
			"body": {"type": "string"}
		},
		"required": ["action"]
	}`))
	return s
}

func graphBase() string {
	if testGraphBase != "" {
		return testGraphBase
	}
	return graphAPIBase
}

func graphGET(ctx context.Context, tok, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", graphBase()+path, nil)
	if err != nil {
		return tool.Error("graph request: " + err.Error()), nil
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return doGraph(req)
}

func graphPOST(ctx context.Context, tok, path string, body map[string]any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return tool.Error("marshal graph body: " + err.Error()), nil
	}
	req, err := http.NewRequestWithContext(ctx, "POST", graphBase()+path, bytes.NewReader(payload))
	if err != nil {
		return tool.Error("graph request: " + err.Error()), nil
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	return doGraph(req)
}

func graphPATCH(ctx context.Context, tok, path string, body map[string]any) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return tool.Error("marshal graph body: " + err.Error()), nil
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", graphBase()+path, bytes.NewReader(payload))
	if err != nil {
		return tool.Error("graph request: " + err.Error()), nil
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	return doGraph(req)
}

func graphDELETE(ctx context.Context, tok, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", graphBase()+path, nil)
	if err != nil {
		return tool.Error("graph request: " + err.Error()), nil
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return doGraph(req)
}

func intArg(args map[string]any, key string, fallback int) int {
	if v, ok := args[key]; ok {
		if n, ok := v.(float64); ok {
			return int(n)
		}
	}
	return fallback
}

func doGraph(req *http.Request) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return tool.Error("graph: " + err.Error()), nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return tool.Error("graph read: " + err.Error()), nil
	}
	if resp.StatusCode >= 400 {
		return tool.Error(fmt.Sprintf("graph HTTP %d: %s", resp.StatusCode, raw)), nil
	}
	return string(raw), nil
}
