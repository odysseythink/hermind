package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, Role("user"), RoleUser)
	assert.Equal(t, Role("assistant"), RoleAssistant)
	assert.Equal(t, Role("tool"), RoleTool)
	assert.Equal(t, Role("system"), RoleSystem)
}

func TestMessageJSONRoundtripText(t *testing.T) {
	msg := Message{
		Role:    RoleUser,
		Content: TextContent("hello world"),
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, RoleUser, decoded.Role)
	assert.True(t, decoded.Content.IsText())
	assert.Equal(t, "hello world", decoded.Content.Text())
}

func TestUsageZeroValue(t *testing.T) {
	var u Usage
	assert.Equal(t, 0, u.InputTokens)
	assert.Equal(t, 0, u.OutputTokens)
}
