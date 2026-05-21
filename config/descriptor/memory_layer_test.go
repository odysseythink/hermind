package descriptor

import "testing"

func TestMemoryLayerDescriptor_Registered(t *testing.T) {
	s, ok := Get("memory_layer")
	if !ok {
		t.Fatal("memory_layer section not registered")
	}
	if s.Key != "memory_layer" {
		t.Errorf("Key = %q, want %q", s.Key, "memory_layer")
	}
	if s.Label != "Memory Layer" {
		t.Errorf("Label = %q, want %q", s.Label, "Memory Layer")
	}
	if s.GroupID != "memory" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "memory")
	}
	if len(s.Fields) == 0 {
		t.Fatal("no fields registered")
	}
}

func TestMemoryLayerDescriptor_HasExpectedFields(t *testing.T) {
	s, _ := Get("memory_layer")
	want := []string{
		"hybrid.enabled",
		"hybrid.rrf_k",
		"hybrid.bm25_top_n_multiplier",
		"hybrid.vector_top_n_multiplier",
		"hybrid.pre_rerank_top_k_multiplier",
		"hybrid.reinforcement_alpha",
		"hybrid.neglect_penalty",
		"reranker.enabled",
		"reranker.batch_size",
		"reranker.timeout_ms",
		"boundary.hard_token_limit",
		"boundary.hard_turn_limit",
		"boundary.soft_token_threshold",
		"boundary.idle_gap_minutes",
		"boundary.topic_shift_enabled",
		"boundary.topic_shift_cosine_threshold",
		"taxonomy.enabled",
		"taxonomy.max_outputs",
		"taxonomy.timeout_ms",
		"taxonomy.types",
		"agentic.enabled",
		"agentic.max_extra_rounds",
		"agentic.expansion_queries",
		"agentic.shortcut_threshold",
		"agentic.per_turn_token_cap",
		"agentic.per_session_token_cap",
		"agentic.timeout_ms",
		"lifecycle.inject_core_on_start",
		"lifecycle.core_max_count",
		"lifecycle.core_max_tokens",
		"lifecycle.inject_foresight_on_start",
		"lifecycle.foresight_max_count",
		"lifecycle.foresight_days_ahead",
		"lifecycle.inject_profile_on_start",
		"lifecycle.profile_max_tokens",
		"lifecycle.profile_user_id",
		"profile.enabled",
		"profile.timeout_ms",
		"profile.max_sections",
		"profile.default_user_id",
		"skill_emitter.enabled",
		"skill_emitter.max_turns",
		"recall_limit",
	}
	names := make(map[string]bool, len(s.Fields))
	for _, f := range s.Fields {
		names[f.Name] = true
	}
	for _, w := range want {
		if !names[w] {
			t.Errorf("missing expected field %q", w)
		}
	}
	if len(s.Fields) != len(want) {
		t.Errorf("got %d fields, want %d", len(s.Fields), len(want))
	}
}

func TestMemoryLayerDescriptor_VisibleWhenReferencesValidField(t *testing.T) {
	s, _ := Get("memory_layer")
	nameSet := make(map[string]bool, len(s.Fields))
	for _, f := range s.Fields {
		nameSet[f.Name] = true
	}
	for _, f := range s.Fields {
		if f.VisibleWhen == nil {
			continue
		}
		if !nameSet[f.VisibleWhen.Field] {
			t.Errorf("field %q references non-existent VisibleWhen.Field %q", f.Name, f.VisibleWhen.Field)
		}
	}
}

func TestMemoryLayerDescriptor_TaxonomyTypesIsMultiSelect(t *testing.T) {
	s, _ := Get("memory_layer")
	for _, f := range s.Fields {
		if f.Name != "taxonomy.types" {
			continue
		}
		if f.Kind != FieldMultiSelect {
			t.Errorf("taxonomy.types Kind = %v, want FieldMultiSelect", f.Kind)
		}
		if len(f.Enum) == 0 {
			t.Error("taxonomy.types has empty Enum")
		}
		for _, want := range []string{"core", "episode", "fact", "foresight"} {
			found := false
			for _, e := range f.Enum {
				if e == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("taxonomy.types Enum missing %q", want)
			}
		}
	}
}
