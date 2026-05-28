package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateArgs_NoSchema(t *testing.T) {
	err := validateArgsAgainstSchema(map[string]any{"foo": "bar"}, nil)
	assert.NoError(t, err)
}

func TestValidateArgs_EmptySchema(t *testing.T) {
	err := validateArgsAgainstSchema(map[string]any{"foo": "bar"}, json.RawMessage("{}"))
	assert.NoError(t, err)
}

func TestValidateArgs_RequiredFieldMissing(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["text"],"properties":{"text":{"type":"string"}}}`)
	err := validateArgsAgainstSchema(map[string]any{}, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "text")
	assert.Contains(t, err.Error(), "required")
}

func TestValidateArgs_WrongType(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)
	err := validateArgsAgainstSchema(map[string]any{"text": 123}, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "string")
}

func TestValidateArgs_AdditionalProperties(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"text":{"type":"string"}}}`)
	err := validateArgsAgainstSchema(map[string]any{"text": "ok", "extra": 1}, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extra")
}

func TestValidateArgs_MultipleErrorsAggregated(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["a","b"],"properties":{"a":{"type":"string"},"b":{"type":"number"}}}`)
	err := validateArgsAgainstSchema(map[string]any{}, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a")
	assert.Contains(t, err.Error(), "b")
}

func TestValidateArgs_NestedObject(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"foo":{"type":"object","properties":{"bar":{"type":"string"}},"required":["bar"]}}}`)
	err := validateArgsAgainstSchema(map[string]any{"foo": map[string]any{}}, schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bar")
}

// ── timeout parser tests (Task 4, written here to keep validation together) ──

func TestParseTimeoutParam_Empty(t *testing.T) {
	d, err := parseTimeoutParam("")
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), d)
}

func TestParseTimeoutParam_Valid_30s(t *testing.T) {
	d, err := parseTimeoutParam("30s")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)
}

func TestParseTimeoutParam_Valid_2m(t *testing.T) {
	d, err := parseTimeoutParam("2m")
	require.NoError(t, err)
	assert.Equal(t, 2*time.Minute, d)
}

func TestParseTimeoutParam_TooSmall(t *testing.T) {
	_, err := parseTimeoutParam("500ms")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestParseTimeoutParam_TooLarge(t *testing.T) {
	_, err := parseTimeoutParam("301s")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestParseTimeoutParam_NotDuration(t *testing.T) {
	_, err := parseTimeoutParam("abc")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
