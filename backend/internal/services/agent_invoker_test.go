package services_test

import (
	"github.com/odysseythink/hermind/backend/internal/agent"
	"github.com/odysseythink/hermind/backend/internal/services"
)

// Compile-time check: agent.Runtime satisfies services.AgentInvoker.
var _ services.AgentInvoker = (*agent.Runtime)(nil)
