package descriptor

// MemoryLayer mirrors config.MemoryLayerConfig. Each subsystem (hybrid,
// reranker, boundary, taxonomy, agentic, lifecycle, profile, skill_emitter)
// is gated by its own "enabled" switch so the UI can collapse advanced
// knobs when a subsystem is turned off.
func init() {
	enabledGate := func(field string) *Predicate {
		return &Predicate{Field: field, Equals: true}
	}

	Register(Section{
		Key:     "memory_layer",
		Label:   "Memory Layer",
		Summary: "Intelligent memory middleware: hybrid retrieval, agentic recall, boundary-driven extraction, living profile, and skill candidate emission.",
		GroupID: "memory",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			// ── Hybrid ──
			{
				Name:    "hybrid.enabled",
				Label:   "Hybrid retrieval enabled",
				Help:    "Fuse BM25 (FTS5) and vector search via Reciprocal Rank Fusion, then apply reinforcement signals.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "hybrid.rrf_k",
				Label:       "RRF constant (k)",
				Help:        "Reciprocal Rank Fusion constant. Higher values dampen rank differences.",
				Kind:        FieldFloat,
				Default:     60.0,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},
			{
				Name:        "hybrid.bm25_top_n_multiplier",
				Label:       "BM25 top-N multiplier",
				Help:        "How many BM25 candidates to fetch relative to the final limit (limit * N).",
				Kind:        FieldInt,
				Default:     3,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},
			{
				Name:        "hybrid.vector_top_n_multiplier",
				Label:       "Vector top-N multiplier",
				Help:        "How many vector candidates to fetch relative to the final limit (limit * N).",
				Kind:        FieldInt,
				Default:     3,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},
			{
				Name:        "hybrid.pre_rerank_top_k_multiplier",
				Label:       "Pre-rerank top-K multiplier",
				Help:        "How many fused candidates to keep before sending to the LLM reranker (limit * N).",
				Kind:        FieldInt,
				Default:     2,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},
			{
				Name:        "hybrid.reinforcement_alpha",
				Label:       "Reinforcement boost (α)",
				Help:        "Score multiplier for memories with positive reinforcement signal. 0 disables.",
				Kind:        FieldFloat,
				Default:     0.15,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},
			{
				Name:        "hybrid.neglect_penalty",
				Label:       "Neglect penalty",
				Help:        "Score reduction for memories with high neglect count. 0 disables.",
				Kind:        FieldFloat,
				Default:     0.10,
				VisibleWhen: enabledGate("hybrid.enabled"),
			},

			// ── Reranker ──
			{
				Name:    "reranker.enabled",
				Label:   "LLM reranker enabled",
				Help:    "Re-rank fused candidates with a lightweight LLM call. Failure degrades silently.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "reranker.batch_size",
				Label:       "Reranker batch size",
				Help:        "Maximum candidates sent to the LLM in one rerank call.",
				Kind:        FieldInt,
				Default:     20,
				VisibleWhen: enabledGate("reranker.enabled"),
			},
			{
				Name:        "reranker.timeout_ms",
				Label:       "Reranker timeout (ms)",
				Help:        "Milliseconds before the reranker call is cancelled and the input order is used.",
				Kind:        FieldInt,
				Default:     1500,
				VisibleWhen: enabledGate("reranker.enabled"),
			},

			// ── Boundary ──
			{
				Name:    "boundary.hard_token_limit",
				Label:   "Boundary hard token limit",
				Help:    "Maximum tokens in the turn buffer before a boundary is forced.",
				Kind:    FieldInt,
				Default: 8000,
			},
			{
				Name:    "boundary.hard_turn_limit",
				Label:   "Boundary hard turn limit",
				Help:    "Maximum turns in the buffer before a boundary is forced.",
				Kind:    FieldInt,
				Default: 20,
			},
			{
				Name:    "boundary.soft_token_threshold",
				Label:   "Boundary soft token threshold",
				Help:    "Minimum tokens before topic-shift detection is allowed.",
				Kind:    FieldInt,
				Default: 1500,
			},
			{
				Name:    "boundary.idle_gap_minutes",
				Label:   "Boundary idle gap (minutes)",
				Help:    "Minutes of inactivity that triggers a boundary.",
				Kind:    FieldInt,
				Default: 10,
			},
			{
				Name:    "boundary.topic_shift_enabled",
				Label:   "Topic shift detection enabled",
				Help:    "Detect topic drift via cosine similarity between buffer head and tail embeddings.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "boundary.topic_shift_cosine_threshold",
				Label:       "Topic shift cosine threshold",
				Help:        "Cosine similarity below this value signals a topic shift. Only checked when soft threshold is met.",
				Kind:        FieldFloat,
				Default:     0.55,
				VisibleWhen: enabledGate("boundary.topic_shift_enabled"),
			},

			// ── Taxonomy ──
			{
				Name:    "taxonomy.enabled",
				Label:   "Taxonomy extraction enabled",
				Help:    "After a boundary, ask the LLM to classify durable memories into core, episode, fact, or foresight.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "taxonomy.max_outputs",
				Label:       "Max extracted memories per boundary",
				Help:        "Hard cap on how many memories the LLM may emit per boundary.",
				Kind:        FieldInt,
				Default:     8,
				VisibleWhen: enabledGate("taxonomy.enabled"),
			},
			{
				Name:        "taxonomy.timeout_ms",
				Label:       "Taxonomy extraction timeout (ms)",
				Help:        "Milliseconds before the extraction LLM call is cancelled.",
				Kind:        FieldInt,
				Default:     6000,
				VisibleWhen: enabledGate("taxonomy.enabled"),
			},
			{
				Name:        "taxonomy.types",
				Label:       "Allowed memory types",
				Help:        "Which taxonomy types the extractor may emit. Empty means all four.",
				Kind:        FieldMultiSelect,
				Default:     []string{"core", "episode", "fact", "foresight"},
				Enum:        []string{"core", "episode", "fact", "foresight"},
				VisibleWhen: enabledGate("taxonomy.enabled"),
			},

			// ── Agentic ──
			{
				Name:    "agentic.enabled",
				Label:   "Agentic multi-round enabled",
				Help:    "When the first recall is weak, run a critic-driven extra round with sub-queries.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "agentic.max_extra_rounds",
				Label:       "Max extra rounds",
				Help:        "How many additional retrieval rounds the agentic wrapper may perform.",
				Kind:        FieldInt,
				Default:     1,
				VisibleWhen: enabledGate("agentic.enabled"),
			},
			{
				Name:        "agentic.expansion_queries",
				Label:       "Expansion query count",
				Help:        "How many complementary sub-queries the critic generates per extra round.",
				Kind:        FieldInt,
				Default:     2,
				VisibleWhen: enabledGate("agentic.enabled"),
			},
			{
				Name:        "agentic.shortcut_threshold",
				Label:       "Shortcut threshold",
				Help:        "If the top candidate's score exceeds this, skip the critic entirely.",
				Kind:        FieldFloat,
				Default:     0.85,
				VisibleWhen: enabledGate("agentic.enabled"),
			},
			{
				Name:        "agentic.per_turn_token_cap",
				Label:       "Per-turn token cap",
				Help:        "Maximum extra tokens the agentic wrapper may spend in a single turn.",
				Kind:        FieldInt,
				Default:     2000,
				VisibleWhen: enabledGate("agentic.enabled"),
			},
			{
				Name:        "agentic.per_session_token_cap",
				Label:       "Per-session token cap",
				Help:        "Maximum extra tokens the agentic wrapper may spend across the whole session.",
				Kind:        FieldInt,
				Default:     20000,
				VisibleWhen: enabledGate("agentic.enabled"),
			},
			{
				Name:        "agentic.timeout_ms",
				Label:       "Agentic timeout (ms)",
				Help:        "Milliseconds before the entire agentic pass falls back to the first-round result.",
				Kind:        FieldInt,
				Default:     8000,
				VisibleWhen: enabledGate("agentic.enabled"),
			},

			// ── Lifecycle ──
			{
				Name:    "lifecycle.inject_core_on_start",
				Label:   "Inject core memories on session start",
				Help:    "Preload 'core' memories into every new session's pinned context.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "lifecycle.core_max_count",
				Label:       "Core memory max count",
				Help:        "Maximum number of core memory rows to preload.",
				Kind:        FieldInt,
				Default:     10,
				VisibleWhen: enabledGate("lifecycle.inject_core_on_start"),
			},
			{
				Name:        "lifecycle.core_max_tokens",
				Label:       "Core memory max tokens",
				Help:        "Character-based cap on the total size of injected core memories.",
				Kind:        FieldInt,
				Default:     600,
				VisibleWhen: enabledGate("lifecycle.inject_core_on_start"),
			},
			{
				Name:    "lifecycle.inject_foresight_on_start",
				Label:   "Inject foresight on session start",
				Help:    "Preload near-term foresight memories (not yet expired) into pinned context.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "lifecycle.foresight_max_count",
				Label:       "Foresight max count",
				Help:        "Maximum foresight entries to inject per session start.",
				Kind:        FieldInt,
				Default:     3,
				VisibleWhen: enabledGate("lifecycle.inject_foresight_on_start"),
			},
			{
				Name:        "lifecycle.foresight_days_ahead",
				Label:       "Foresight lookahead (days)",
				Help:        "Only inject foresights whose expiration is within this many days.",
				Kind:        FieldInt,
				Default:     7,
				VisibleWhen: enabledGate("lifecycle.inject_foresight_on_start"),
			},
			{
				Name:    "lifecycle.inject_profile_on_start",
				Label:   "Inject user profile on session start",
				Help:    "Preload the living user profile as a ## User Profile block into pinned context.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "lifecycle.profile_max_tokens",
				Label:       "Profile max tokens",
				Help:        "Character-based cap on the size of the injected profile block.",
				Kind:        FieldInt,
				Default:     800,
				VisibleWhen: enabledGate("lifecycle.inject_profile_on_start"),
			},
			{
				Name:        "lifecycle.profile_user_id",
				Label:       "Profile user ID",
				Help:        "User identifier for the living profile (single-user installs can leave as 'default').",
				Kind:        FieldString,
				Default:     "default",
				VisibleWhen: enabledGate("lifecycle.inject_profile_on_start"),
			},

			// ── Profile updater ──
			{
				Name:    "profile.enabled",
				Label:   "Living profile enabled",
				Help:    "After each boundary, ask the LLM to incrementally update the user's structured profile.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "profile.timeout_ms",
				Label:       "Profile update timeout (ms)",
				Help:        "Milliseconds before the profile-update LLM call is cancelled.",
				Kind:        FieldInt,
				Default:     6000,
				VisibleWhen: enabledGate("profile.enabled"),
			},
			{
				Name:        "profile.max_sections",
				Label:       "Profile max sections",
				Help:        "Maximum existing profile sections rendered into the update prompt.",
				Kind:        FieldInt,
				Default:     24,
				VisibleWhen: enabledGate("profile.enabled"),
			},
			{
				Name:        "profile.default_user_id",
				Label:       "Profile default user ID",
				Help:        "Fallback user ID when turns don't carry one.",
				Kind:        FieldString,
				Default:     "default",
				VisibleWhen: enabledGate("profile.enabled"),
			},

			// ── Skill emitter ──
			{
				Name:    "skill_emitter.enabled",
				Label:   "Skill candidate emitter enabled",
				Help:    "After each boundary, emit a candidate signal to the skills Evolver for possible skill extraction.",
				Kind:    FieldBool,
				Default: true,
			},
			{
				Name:        "skill_emitter.max_turns",
				Label:       "Skill emitter max turns",
				Help:        "Maximum turns from the boundary included in the skill candidate payload.",
				Kind:        FieldInt,
				Default:     8,
				VisibleWhen: enabledGate("skill_emitter.enabled"),
			},

			// ── Global ──
			{
				Name:    "recall_limit",
				Label:   "Recall limit",
				Help:    "Default number of memories returned to the engine per turn.",
				Kind:    FieldInt,
				Default: 5,
			},
		},
	})
}
