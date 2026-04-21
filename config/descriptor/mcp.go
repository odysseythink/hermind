package descriptor

// MCP mirrors config.MCPConfig. YAML layout: mcp.servers.<name>.{command,...}.
// Each server keyed by its name (ShapeKeyedMap) with uniform fields
// (NoDiscriminator — no provider/type field; instance name IS the identity).
//
// Args ([]string) and Env (map[string]string) are NOT exposed in the UI
// yet — those need new FieldStringList / FieldStringMap kinds. Users
// with complex MCP setups should edit config.yaml directly for now; the
// UI still lets them toggle Enabled and rename Command.
func init() {
	Register(Section{
		Key:             "mcp",
		Label:           "MCP servers",
		Summary:         "Model Context Protocol servers launched on CLI/gateway startup.",
		GroupID:         "advanced",
		Shape:           ShapeKeyedMap,
		Subkey:          "servers",
		NoDiscriminator: true,
		Fields: []FieldSpec{
			{Name: "command", Label: "Command", Kind: FieldString, Required: true,
				Help: "Executable to run, e.g. \"npx\" or a full path."},
			{Name: "enabled", Label: "Enabled", Kind: FieldBool,
				Help: "Disabled servers never start. Delete the entry to remove it entirely. Edit config.yaml directly to configure args/env."},
		},
	})
}
