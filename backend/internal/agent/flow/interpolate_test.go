package flow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterpolate_SingleVar_Replaced(t *testing.T) {
	out := Interpolate("hello {{name}}", map[string]string{"name": "world"})
	require.Equal(t, "hello world", out)
}

func TestInterpolate_MissingVar_LeftAsIs(t *testing.T) {
	out := Interpolate("hello {{name}}", map[string]string{})
	require.Equal(t, "hello {{name}}", out)
}

func TestInterpolate_MultipleVars_AllReplaced(t *testing.T) {
	out := Interpolate("{{greeting}} {{name}}!", map[string]string{
		"greeting": "hi",
		"name":     "alice",
	})
	require.Equal(t, "hi alice!", out)
}

func TestInterpolate_LastStepDotOutput_Reads__last_output(t *testing.T) {
	out := Interpolate("result: {{lastStep.output}}", map[string]string{
		"__last_output": "42",
	})
	require.Equal(t, "result: 42", out)
}

func TestInterpolate_NestedBraces_OnlyOuterMatched(t *testing.T) {
	out := Interpolate("{{a{{b}}c}}", map[string]string{"a": "X", "b": "Y"})
	// Only {{b}} matches; {{a and c}} remain
	require.Equal(t, "{{aYc}}", out)
}

func TestParseStep_ValidApiCall_ReturnsStruct(t *testing.T) {
	raw := map[string]any{
		"type": "apiCall",
		"config": map[string]any{
			"url":             "http://example.com",
			"resultVariable":  "myResult",
		},
	}
	step, err := ParseStep(raw)
	require.NoError(t, err)
	require.Equal(t, "apiCall", step.Type)
	require.Equal(t, "myResult", step.ResultVar)
}

func TestParseStep_UnknownType_ReturnsError(t *testing.T) {
	_, err := ParseStep(map[string]any{"config": map[string]any{}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing type")
}
