package agent

import "github.com/nousresearch/hermes-agent/message"

// ModelRates is the per-million-token price for one model.
type ModelRates struct {
	Input         float64
	Output        float64
	CacheRead     float64
	CacheWrite    float64
	ReasoningRate float64
}

// defaultRateTable is a minimal built-in price list (USD per 1M tokens).
// Numbers are approximate list prices as of early 2026 and are only
// used when the caller doesn't supply a custom table.
var defaultRateTable = map[string]ModelRates{
	"claude-opus-4-6":   {Input: 15, Output: 75, CacheRead: 1.5, CacheWrite: 18.75},
	"claude-sonnet-4-6": {Input: 3, Output: 15, CacheRead: 0.3, CacheWrite: 3.75},
	"claude-haiku-4-5":  {Input: 0.25, Output: 1.25, CacheRead: 0.03, CacheWrite: 0.3},
	"gpt-4o":            {Input: 2.5, Output: 10, CacheRead: 1.25},
	"gpt-4o-mini":       {Input: 0.15, Output: 0.6, CacheRead: 0.075},
	"o1":                {Input: 15, Output: 60},
	"o3-mini":           {Input: 1.1, Output: 4.4},
	"deepseek-chat":     {Input: 0.27, Output: 1.1},
	"deepseek-reasoner": {Input: 0.55, Output: 2.19},
}

// ComputeCost returns the USD cost of one Usage record at the given
// per-million-token rate. Pass the default table or a custom one.
func ComputeCost(model string, usage message.Usage, table map[string]ModelRates) float64 {
	if table == nil {
		table = defaultRateTable
	}
	rates, ok := table[model]
	if !ok {
		return 0
	}
	const million = 1_000_000.0
	cost := 0.0
	cost += float64(usage.InputTokens) * rates.Input / million
	cost += float64(usage.OutputTokens) * rates.Output / million
	cost += float64(usage.CacheReadTokens) * rates.CacheRead / million
	cost += float64(usage.CacheWriteTokens) * rates.CacheWrite / million
	cost += float64(usage.ReasoningTokens) * rates.ReasoningRate / million
	return cost
}
