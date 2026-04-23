package descriptor

func init() {
	Register(Section{
		Key:              "gateway",
		Label:            "IM Channels",
		Summary:          "Multi-platform IM adapters (Feishu, Telegram, …)",
		GroupID:          "advanced",
		Shape:            ShapeKeyedMap,
		Subkey:           "platforms",
		NoDiscriminator:  true,
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
			{
				Name:  "options",
				Label: "Platform Options",
				Kind:  FieldText,
				Help:  "JSON or YAML config. Edit config.yaml directly for advanced setups.",
			},
		},
	})
}
