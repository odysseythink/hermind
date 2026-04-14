package agent

import (
	"strings"
	"testing"

	"github.com/odysseythink/hermind/message"
)

func TestComputeCost(t *testing.T) {
	usage := message.Usage{InputTokens: 1_000_000, OutputTokens: 500_000}
	cost := ComputeCost("claude-opus-4-6", usage, nil)
	want := 15.0 + 0.5*75 // 15 + 37.5 = 52.5
	if cost < want-0.001 || cost > want+0.001 {
		t.Errorf("cost = %f, want %f", cost, want)
	}
}

func TestComputeCostUnknownModel(t *testing.T) {
	if c := ComputeCost("no-such-model", message.Usage{InputTokens: 1000}, nil); c != 0 {
		t.Errorf("expected 0, got %f", c)
	}
}

func TestSelectModelCode(t *testing.T) {
	m, kind := SelectModel("please refactor this function", "default-model", nil)
	if kind != "code" {
		t.Errorf("kind = %q", kind)
	}
	if m != "claude-opus-4-6" {
		t.Errorf("model = %q", m)
	}
}

func TestSelectModelFallback(t *testing.T) {
	m, kind := SelectModel("what is the weather", "default-model", nil)
	if kind != "default" || m != "default-model" {
		t.Errorf("got (%s, %s)", m, kind)
	}
}

func TestCredentialPoolRoundRobin(t *testing.T) {
	p, err := NewCredentialPool([]string{"k1", "k2", "k3"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for i := 0; i < 6; i++ {
		seen[p.Next()]++
	}
	if seen["k1"] != 2 || seen["k2"] != 2 || seen["k3"] != 2 {
		t.Errorf("uneven distribution: %v", seen)
	}
	p.Disable("k2")
	seen = map[string]int{}
	for i := 0; i < 4; i++ {
		seen[p.Next()]++
	}
	if seen["k2"] != 0 {
		t.Errorf("k2 should be disabled: %v", seen)
	}
	if p.Available() != 2 {
		t.Errorf("available = %d", p.Available())
	}
	p.Reset()
	if p.Available() != 3 {
		t.Errorf("reset available = %d", p.Available())
	}
}

func TestRedact(t *testing.T) {
	cases := []string{
		"here is my token: sk-ant-abcdefghijklmnopqrstuvwxyz1234567890",
		"contact me at foo@example.com",
		"authorization: Bearer abc123abc123abc123abc123abc123",
		"AKIAIOSFODNN7EXAMPLE is the key",
	}
	for _, c := range cases {
		out := Redact(c)
		if !strings.Contains(out, "[REDACTED]") {
			t.Errorf("missing [REDACTED] in %q", out)
		}
	}
}
