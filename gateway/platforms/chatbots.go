package platforms

import "github.com/odysseythink/hermind/gateway"

// NewSlack builds a Slack incoming-webhook bot. Expected shape:
// {"text":"..."}. Paired with api_server for inbound.
func NewSlack(url string) *WebhookBot {
	return NewWebhookBot("slack", url, func(out gateway.OutgoingMessage) any {
		return map[string]string{"text": out.Text}
	})
}

// NewDiscord builds a Discord incoming-webhook bot. Expected shape:
// {"content":"..."}.
func NewDiscord(url string) *WebhookBot {
	return NewWebhookBot("discord", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{"content": out.Text}
	})
}

// NewMattermost builds a Mattermost incoming-webhook bot. Expected
// shape: {"text":"..."}.
func NewMattermost(url string) *WebhookBot {
	return NewWebhookBot("mattermost", url, func(out gateway.OutgoingMessage) any {
		return map[string]string{"text": out.Text}
	})
}

// NewDingTalk builds a DingTalk incoming-webhook bot.
// Expected shape: {"msgtype":"text","text":{"content":"..."}}.
func NewDingTalk(url string) *WebhookBot {
	return NewWebhookBot("dingtalk", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": out.Text},
		}
	})
}

// NewWeCom builds a WeCom (enterprise WeChat) incoming-webhook bot.
// Expected shape: {"msgtype":"text","text":{"content":"..."}}.
func NewWeCom(url string) *WebhookBot {
	return NewWebhookBot("wecom", url, func(out gateway.OutgoingMessage) any {
		return map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": out.Text},
		}
	})
}
