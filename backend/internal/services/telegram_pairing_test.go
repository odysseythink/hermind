package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramBotService_PairingFlow(t *testing.T) {
	svc := NewTelegramBotService(nil, nil, nil, nil, nil)

	// Simulate /start
	svc.pending.Store("123", &pendingPairing{Code: "000123", Username: "alice", FirstName: "Alice"})

	users := svc.PendingUsers()
	require.Len(t, users, 1)
	assert.Equal(t, "123", users[0].ChatID)

	// Approve
	ctx := context.Background()
	err := svc.ApproveUser(ctx, "123", "alice")
	require.NoError(t, err)

	assert.Empty(t, svc.PendingUsers())
	approved := svc.ApprovedUsers()
	require.Len(t, approved, 1)
	assert.Equal(t, "alice", approved[0].Username)
}
