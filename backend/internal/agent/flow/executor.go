package flow

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
)

// Executor runs agent flows step by step.
type Executor struct {
	lm           core.LanguageModel
	httpClient   *http.Client
	allowPrivate bool
}

// New creates an Executor.
func New(lm core.LanguageModel, allowPrivateIPs bool) *Executor {
	return &Executor{
		lm:           lm,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		allowPrivate: allowPrivateIPs,
	}
}

// Run executes the flow sequentially, returning the final step output.
func (e *Executor) Run(ctx context.Context, flow *services.LoadedFlow, initialVars map[string]string, emit func(string)) (string, error) {
	if emit == nil {
		emit = func(string) {}
	}
	if initialVars == nil {
		initialVars = map[string]string{}
	}
	fc := &Context{
		Variables:       initialVars,
		Emit:            emit,
		LM:              e.lm,
		HTTPClient:      e.httpClient,
		AllowPrivateIPs: e.allowPrivate,
	}
	for i, raw := range flow.Config.Steps {
		step, err := ParseStep(raw)
		if err != nil {
			return "", fmt.Errorf("step %d parse: %w", i, err)
		}
		fc.Emit(fmt.Sprintf("Flow step %d/%d: %s", i+1, len(flow.Config.Steps), step.Type))
		output, err := e.runStep(ctx, fc, step)
		if err != nil {
			return "", fmt.Errorf("step %d (%s): %w", i, step.Type, err)
		}
		if step.ResultVar != "" {
			fc.Variables[step.ResultVar] = output
		}
		fc.Variables["__last_output"] = output
	}
	return fc.Variables["__last_output"], nil
}

func (e *Executor) runStep(ctx context.Context, fc *Context, step *Step) (string, error) {
	switch step.Type {
	case "apiCall":
		return ExecuteAPICall(ctx, fc, step.Config)
	case "llmInstruction":
		return ExecuteLLMInstruction(ctx, fc, step.Config)
	case "webScraping":
		return ExecuteWebScraping(ctx, fc, step.Config)
	default:
		return "", fmt.Errorf("unknown block type: %s", step.Type)
	}
}
