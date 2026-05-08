package api

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestMetaClawConfigJudgeFields(t *testing.T) {
	c := config.MetaClawConfig{
		JudgeEnabled: true,
		SummaryEvery: 7,
	}
	if !c.JudgeEnabled {
		t.Fatal("JudgeEnabled not round-tripping")
	}
	if c.SummaryEvery != 7 {
		t.Fatal("SummaryEvery not round-tripping")
	}
}

func TestMemoryConfigConsolidatorFields(t *testing.T) {
	c := config.MemoryConfig{
		ConsolidateIntervalSeconds:  120,
		ConsolidateIdleAfterSeconds: 60,
	}
	if c.ConsolidateIntervalSeconds != 120 {
		t.Fatal("interval not round-tripping")
	}
	if c.ConsolidateIdleAfterSeconds != 60 {
		t.Fatal("idle_after not round-tripping")
	}
}
