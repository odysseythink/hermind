package message

import (
	"fmt"

	"github.com/odysseythink/pantheon/core"
)

// Type aliases — hermind Message is now exactly pantheon core.Message.
type (
	HermindMessage = core.Message
)

// ---- content helpers ----

// ---- adapter (legacy compatibility) ----

// ToPantheon converts a hermind Message to a pantheon core.Message.
// Since the types are identical this is almost a no-op, but it preserves
// the legacy compatibility fix that rewrites core.MESSAGE_ROLE_USER → core.MESSAGE_ROLE_TOOL when the
// message carries tool results.
func ToPantheon(m HermindMessage) core.Message {
	role := m.Role
	origRole := role

	if role == core.MESSAGE_ROLE_USER && hasToolResultPart(m.Content) {
		role = core.MESSAGE_ROLE_TOOL
	}
	if origRole != role {
		fmt.Printf("[ToPantheon] CONVERTED role %s -> %s parts=%d\n", origRole, role, len(m.Content))
		for i, p := range m.Content {
			fmt.Printf("[ToPantheon]   part[%d] type=%T\n", i, p)
		}
	}

	return core.Message{
		Role:       core.MessageRoleType(role),
		Content:    m.Content,
		Name:       m.Name,
		ToolCallID: m.ToolCallID,
	}
}

func hasToolResultPart(parts []core.ContentParter) bool {
	for _, p := range parts {
		if _, ok := p.(core.ToolResultPart); ok {
			return true
		}
	}
	return false
}

// MessageFromPantheon converts a pantheon core.Message to a hermind Message.
func MessageFromPantheon(m core.Message) HermindMessage {
	return HermindMessage(m)
}
