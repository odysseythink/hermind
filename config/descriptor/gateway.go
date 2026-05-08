package descriptor

func init() {
	gate := func(values ...string) *Predicate {
		if len(values) == 1 {
			return &Predicate{Field: "type", Equals: values[0]}
		}
		ins := make([]any, len(values))
		for i, v := range values {
			ins[i] = v
		}
		return &Predicate{Field: "type", In: ins}
	}
	Register(Section{
		Key:             "gateway",
		Label:           "IM Channels",
		Summary:         "Multi-platform IM adapters (Feishu, Telegram, Slack, Discord, WeChat, DingTalk)",
		GroupID:         "gateway",
		Shape:           ShapeKeyedMap,
		Subkey:          "platforms",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{
				Name:     "type",
				Label:    "Platform Type",
				Kind:     FieldEnum,
				Required: true,
				Enum:     []string{"feishu", "telegram", "slack", "discord", "wechat", "dingtalk"},
			},
			{
				Name:    "enabled",
				Label:   "Enabled",
				Kind:    FieldBool,
				Default: true,
			},

			// --- Telegram ---
			{
				Name:        "options.bot_token",
				Label:       "Bot Token",
				Kind:        FieldSecret,
				Required:    true,
				Help:        "Bot API token from @BotFather.",
				VisibleWhen: gate("telegram", "slack", "discord"),
			},
			{
				Name:        "options.proxy_url",
				Label:       "HTTP Proxy URL",
				Kind:        FieldString,
				Help:        "Optional HTTP proxy (useful in restrictive regions).",
				VisibleWhen: gate("telegram"),
			},

			// --- Feishu (Lark) ---
			{
				Name:        "options.app_id",
				Label:       "App ID",
				Kind:        FieldString,
				Required:    true,
				VisibleWhen: gate("feishu"),
			},
			{
				Name:        "options.app_secret",
				Label:       "App Secret",
				Kind:        FieldSecret,
				Required:    true,
				VisibleWhen: gate("feishu", "dingtalk"),
			},
			{
				Name:        "options.verification_token",
				Label:       "Verification Token",
				Kind:        FieldSecret,
				Help:        "Event webhook verification token.",
				VisibleWhen: gate("feishu"),
			},
			{
				Name:        "options.encrypt_key",
				Label:       "Encrypt Key",
				Kind:        FieldSecret,
				Help:        "Optional message encryption key.",
				VisibleWhen: gate("feishu"),
			},

			// --- Slack ---
			{
				Name:        "options.signing_secret",
				Label:       "Signing Secret",
				Kind:        FieldSecret,
				Help:        "Webhook request signing secret.",
				VisibleWhen: gate("slack"),
			},
			{
				Name:        "options.app_token",
				Label:       "App-Level Token",
				Kind:        FieldSecret,
				Help:        "xapp-... token for Socket Mode.",
				VisibleWhen: gate("slack"),
			},

			// --- Discord ---
			{
				Name:        "options.application_id",
				Label:       "Application ID",
				Kind:        FieldString,
				Help:        "Discord application / client ID.",
				VisibleWhen: gate("discord"),
			},

			// --- WeChat Work (企业微信) ---
			{
				Name:        "options.corp_id",
				Label:       "Corp ID",
				Kind:        FieldString,
				Required:    true,
				VisibleWhen: gate("wechat"),
			},
			{
				Name:        "options.corp_secret",
				Label:       "Corp Secret",
				Kind:        FieldSecret,
				Required:    true,
				VisibleWhen: gate("wechat"),
			},
			{
				Name:        "options.agent_id",
				Label:       "Agent ID",
				Kind:        FieldString,
				Required:    true,
				VisibleWhen: gate("wechat"),
			},
			{
				Name:        "options.token",
				Label:       "Webhook Token",
				Kind:        FieldSecret,
				Help:        "Webhook verification token.",
				VisibleWhen: gate("wechat", "dingtalk"),
			},
			{
				Name:        "options.encoding_aes_key",
				Label:       "Encoding AES Key",
				Kind:        FieldSecret,
				Help:        "Message encryption key (43 characters).",
				VisibleWhen: gate("wechat"),
			},

			// --- DingTalk ---
			{
				Name:        "options.app_key",
				Label:       "App Key",
				Kind:        FieldString,
				Required:    true,
				VisibleWhen: gate("dingtalk"),
			},
			{
				Name:        "options.robot_code",
				Label:       "Robot Code",
				Kind:        FieldString,
				Help:        "Group robot code.",
				VisibleWhen: gate("dingtalk"),
			},
		},
	})
}
