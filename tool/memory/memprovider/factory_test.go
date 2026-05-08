package memprovider

import (
	"testing"

	"github.com/odysseythink/hermind/config"
)

func TestNewFromConfigNoneReturnsNil(t *testing.T) {
	p, err := New(config.MemoryConfig{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil, got %+v", p)
	}
}

func TestNewFromConfigHoncho(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider: "honcho",
		Honcho:   config.HonchoConfig{APIKey: "x"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p == nil || p.Name() != "honcho" {
		t.Errorf("expected honcho provider, got %+v", p)
	}
}

func TestNewFromConfigMem0(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider: "mem0",
		Mem0:     config.Mem0Config{APIKey: "x"},
	})
	if err != nil || p == nil || p.Name() != "mem0" {
		t.Fatalf("expected mem0, got %+v err=%v", p, err)
	}
}

func TestNewFromConfigSupermemory(t *testing.T) {
	p, err := New(config.MemoryConfig{
		Provider:    "supermemory",
		Supermemory: config.SupermemoryConfig{APIKey: "x"},
	})
	if err != nil || p == nil || p.Name() != "supermemory" {
		t.Fatalf("expected supermemory, got %+v err=%v", p, err)
	}
}

func TestNewFromConfigUnknown(t *testing.T) {
	_, err := New(config.MemoryConfig{Provider: "wat"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewFromConfigMissingAPIKey(t *testing.T) {
	_, err := New(config.MemoryConfig{Provider: "honcho"})
	if err == nil {
		t.Fatal("expected error for missing api_key")
	}
}
