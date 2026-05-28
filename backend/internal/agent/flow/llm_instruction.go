package flow

import (
	"context"
	"fmt"

	"github.com/odysseythink/pantheon/core"
)

// ExecuteLLMInstruction sends a prompt to the configured language model.
func ExecuteLLMInstruction(ctx context.Context, fc *Context, config map[string]any) (string, error) {
	instr, _ := config["instruction"].(string)
	if instr == "" {
		return "", fmt.Errorf("instruction is required")
	}
	if fc.LM == nil {
		return "", fmt.Errorf("LLM not available")
	}

	instr = Interpolate(instr, fc.Variables)
	fc.Emit("LLM instruction: " + truncate(instr, 60))

	resp, err := fc.LM.Generate(ctx, &core.Request{
		Messages: []core.Message{core.NewTextMessage(core.MESSAGE_ROLE_USER, instr)},
	})
	if err != nil {
		return "", fmt.Errorf("llm: %w", err)
	}
	return resp.Message.Text(), nil
}
