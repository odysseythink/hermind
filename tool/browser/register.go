package browser

import (
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// RegisterAll registers the browser toolset against reg if the provider
// is configured. Callers pass an already-constructed Provider; if it
// returns false from IsConfigured(), no tools are registered.
func RegisterAll(reg *tool.Registry, p Provider) {
	if p == nil || !p.IsConfigured() {
		return
	}
	store := NewSessionStore()

	reg.Register(&tool.Entry{
		Name:        "browser_session_create",
		Toolset:     "browser",
		Description: "Create a new cloud browser session and return its connect URL.",
		Emoji:       "🌐",
		Handler:     newCreateHandler(p, store),
		Schema: core.ToolDefinition{
			Name:        "browser_session_create",
			Description: "Create a new Browserbase cloud browser session. Returns the session ID, CDP connect URL, and live debug URL.",
			Parameters:  core.MustSchemaFromJSON([]byte(createSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "browser_session_close",
		Toolset:     "browser",
		Description: "Release a browser session.",
		Emoji:       "🧹",
		Handler:     newCloseHandler(p, store),
		Schema: core.ToolDefinition{
			Name:        "browser_session_close",
			Description: "Release a previously created browser session.",
			Parameters:  core.MustSchemaFromJSON([]byte(closeSchema)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "browser_session_live_url",
		Toolset:     "browser",
		Description: "Fetch the live debugger URL for a browser session.",
		Emoji:       "🔭",
		Handler:     newLiveURLHandler(p, store),
		Schema: core.ToolDefinition{
			Name:        "browser_session_live_url",
			Description: "Return the live debugger URL for a browser session so the user or a downstream tool can watch it.",
			Parameters:  core.MustSchemaFromJSON([]byte(liveSchema)),
		},
	})
}
