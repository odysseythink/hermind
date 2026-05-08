package agent

import (
	"sort"
	"sync"
	"time"
)

// Insights aggregates runtime statistics across one session.
type Insights struct {
	mu            sync.Mutex
	ToolCalls     int
	ToolErrors    int
	ToolByName    map[string]int
	ToolDurations map[string][]time.Duration
	Iterations    int
	Started       time.Time
	Ended         time.Time
}

// NewInsights returns a zeroed Insights with Started=now.
func NewInsights() *Insights {
	return &Insights{
		ToolByName:    make(map[string]int),
		ToolDurations: make(map[string][]time.Duration),
		Started:       time.Now().UTC(),
	}
}

// RecordToolCall bumps per-tool counters and duration samples.
func (i *Insights) RecordToolCall(name string, dur time.Duration, err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.ToolCalls++
	if err != nil {
		i.ToolErrors++
	}
	i.ToolByName[name]++
	i.ToolDurations[name] = append(i.ToolDurations[name], dur)
}

// RecordIteration bumps the agent-loop iteration counter.
func (i *Insights) RecordIteration() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Iterations++
}

// Finish stamps Ended.
func (i *Insights) Finish() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.Ended = time.Now().UTC()
}

// TopTools returns the N most-used tool names, sorted desc.
func (i *Insights) TopTools(n int) []string {
	i.mu.Lock()
	defer i.mu.Unlock()
	type kv struct {
		name  string
		count int
	}
	kvs := make([]kv, 0, len(i.ToolByName))
	for k, v := range i.ToolByName {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(a, b int) bool { return kvs[a].count > kvs[b].count })
	if n > len(kvs) {
		n = len(kvs)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, kvs[i].name)
	}
	return out
}

// MeanDuration returns the mean tool duration over all tools.
func (i *Insights) MeanDuration() time.Duration {
	i.mu.Lock()
	defer i.mu.Unlock()
	var total time.Duration
	var count int
	for _, ds := range i.ToolDurations {
		for _, d := range ds {
			total += d
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}
