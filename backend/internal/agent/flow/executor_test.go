package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/stretchr/testify/require"
)

func newTestExecutor(lm core.LanguageModel) *Executor {
	return New(lm, true)
}

func TestExecutor_SingleStep_ApiCall_ReturnsOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("api-result"))
	}))
	defer srv.Close()

	exec := newTestExecutor(nil)
	flow := &services.LoadedFlow{
		Config: services.FlowConfig{
			Steps: []any{
				map[string]any{
					"type": "apiCall",
					"config": map[string]any{
						"url":    srv.URL,
						"method": "GET",
					},
				},
			},
		},
	}
	out, err := exec.Run(context.Background(), flow, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "api-result", out)
}

func TestExecutor_ChainedSteps_VarFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"value":99}`))
	}))
	defer srv.Close()

	lm := &mockLM{
		generateFn: func(ctx context.Context, req *core.Request) (*core.Response, error) {
			prompt := ""
			if len(req.Messages) > 0 {
				prompt = req.Messages[0].Text()
			}
			return &core.Response{Message: core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, "processed: "+prompt)}, nil
		},
	}
	exec := newTestExecutor(lm)
	flow := &services.LoadedFlow{
		Config: services.FlowConfig{
			Steps: []any{
				map[string]any{
					"type": "apiCall",
					"config": map[string]any{
						"url":            srv.URL,
						"method":         "GET",
						"resultVariable": "apiData",
					},
				},
				map[string]any{
					"type": "llmInstruction",
					"config": map[string]any{
						"instruction": "summarise {{apiData}}",
					},
				},
			},
		},
	}
	out, err := exec.Run(context.Background(), flow, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "processed: summarise {\"value\":99}", out)
}

func TestExecutor_StepError_StopsAndReturns(t *testing.T) {
	exec := newTestExecutor(nil)
	f := &services.LoadedFlow{
		Config: services.FlowConfig{
			Steps: []any{
				map[string]any{
					"type": "apiCall",
					"config": map[string]any{
						"url": "http://127.0.0.1:1/fail", // connection refused
					},
				},
			},
		},
	}
	_, err := exec.Run(context.Background(), f, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "step 0")
}

func TestExecutor_NoSteps_ReturnsEmpty(t *testing.T) {
	exec := newTestExecutor(nil)
	flow := &services.LoadedFlow{
		Config: services.FlowConfig{Steps: []any{}},
	}
	out, err := exec.Run(context.Background(), flow, nil, nil)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestExecutor_LastOutputAutoPopulated(t *testing.T) {
	exec := newTestExecutor(nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("step-output"))
	}))
	defer srv.Close()

	flow2 := &services.LoadedFlow{
		Config: services.FlowConfig{
			Steps: []any{
				map[string]any{
					"type": "apiCall",
					"config": map[string]any{
						"url": srv.URL,
					},
				},
			},
		},
	}
	vars := map[string]string{"existing": "var"}
	out, err := exec.Run(context.Background(), flow2, vars, nil)
	require.NoError(t, err)
	require.Equal(t, "step-output", out)
	require.Equal(t, "step-output", vars["__last_output"])
}

func TestExecutor_UnknownBlockType_ReturnsError(t *testing.T) {
	exec := newTestExecutor(nil)
	flow := &services.LoadedFlow{
		Config: services.FlowConfig{
			Steps: []any{
				map[string]any{
					"type":   "unknownBlock",
					"config": map[string]any{},
				},
			},
		},
	}
	_, err := exec.Run(context.Background(), flow, nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown block type")
}
