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
