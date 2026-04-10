// provider/qwen/qwen.go
package qwen

import (
	"github.com/nousresearch/hermes-agent/config"
	"github.com/nousresearch/hermes-agent/provider"
	"github.com/nousresearch/hermes-agent/provider/openaicompat"
)

// defaultBaseURL points to Alibaba DashScope's OpenAI-compatible endpoint.
const defaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

// New constructs a Qwen (通义千问) provider via Alibaba DashScope.
// Popular models: qwen-max, qwen-plus, qwen-turbo, qwen2.5-72b-instruct.
func New(cfg config.ProviderConfig) (provider.Provider, error) {
	return openaicompat.NewFromProviderConfig("qwen", defaultBaseURL, cfg, nil)
}
