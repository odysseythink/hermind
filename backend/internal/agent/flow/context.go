package flow

import (
	"net/http"

	"github.com/odysseythink/pantheon/core"
)

// Context carries execution state across flow steps.
type Context struct {
	Variables       map[string]string
	Emit            func(string)
	LM              core.LanguageModel
	HTTPClient      *http.Client
	AllowPrivateIPs bool
}
