package descriptor

import (
	"github.com/odysseythink/hermind/gateway/platforms"
)

func init() {
	Register(Section{
		Key:        "gateway",
		Label:      "IM Channels",
		Summary:    "Multi-platform IM adapters (Feishu, Telegram, …)",
		GroupID:    "advanced",
		Shape:      ShapeKeyedMap,
		Subkey:     "platforms",
		Fields: []FieldSpec{
			{
				Name:     "type",
				Label:    "Platform Type",
				Kind:     FieldEnum,
				Required: true,
				Enum:     platformTypes(),
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

func platformTypes() []string {
	var types []string
	for _, desc := range platforms.All() {
		types = append(types, desc.Type)
	}
	return types
}
