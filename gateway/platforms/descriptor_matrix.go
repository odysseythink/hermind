package platforms

import "github.com/odysseythink/hermind/gateway"

func init() {
	Register(Descriptor{
		Type:        "matrix",
		DisplayName: "Matrix",
		Summary:     "Matrix client-server API (v3). Joins a single room.",
		Fields: []FieldSpec{
			{Name: "home_server", Label: "Home Server", Kind: FieldString, Required: true,
				Help: `e.g. "https://matrix.org".`},
			{Name: "access_token", Label: "Access Token", Kind: FieldSecret, Required: true},
			{Name: "room_id", Label: "Room ID", Kind: FieldString, Required: true,
				Help: `Internal room id, e.g. "!abcd:matrix.org".`},
		},
		Build: func(opts map[string]string) (gateway.Platform, error) {
			return NewMatrix(opts["home_server"], opts["access_token"], opts["room_id"]), nil
		},
	})
}
